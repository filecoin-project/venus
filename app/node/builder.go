package node

import (
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/venus/app/submodule/dagservice"
	"github.com/filecoin-project/venus/app/submodule/eth"
	"github.com/filecoin-project/venus/app/submodule/f3"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/ipfs-force-community/sophon-auth/core"
	"github.com/ipfs-force-community/sophon-auth/jwtclient"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/app/submodule/actorevent"
	"github.com/filecoin-project/venus/app/submodule/blockstore"
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/common"
	config2 "github.com/filecoin-project/venus/app/submodule/config"
	"github.com/filecoin-project/venus/app/submodule/market"
	"github.com/filecoin-project/venus/app/submodule/mining"
	"github.com/filecoin-project/venus/app/submodule/mpool"
	"github.com/filecoin-project/venus/app/submodule/paych"
	"github.com/filecoin-project/venus/app/submodule/storagenetworking"
	"github.com/filecoin-project/venus/app/submodule/syncer"
	"github.com/filecoin-project/venus/app/submodule/wallet"
	chain2 "github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/journal"
	"github.com/filecoin-project/venus/pkg/paychmgr"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper/impl"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs-force-community/metrics/ratelimit"
)

// Builder is a helper to aid in the construction of a filecoin node.
type Builder struct {
	blockTime      time.Duration
	libp2pOpts     []libp2p.Option
	offlineMode    bool
	verifier       ffiwrapper.Verifier
	propDelay      time.Duration
	repo           repo.Repo
	journal        journal.Journal
	isRelay        bool
	chainClock     clock.ChainEpochClock
	genBlk         types.BlockHeader
	walletPassword []byte
	authURL        string
}

// New creates a new node.
func New(ctx context.Context, opts ...BuilderOpt) (*Node, error) {
	// initialize builder and set base values
	n := &Builder{
		offlineMode: false,
		blockTime:   clock.DefaultEpochDuration,
		verifier:    impl.ProofVerifier,
	}
	// apply builder options
	for _, o := range opts {
		if err := o(n); err != nil {
			return nil, err
		}
	}

	// build the node
	return n.build(ctx)
}

func (b *Builder) build(ctx context.Context) (*Node, error) {
	//
	// Set default values on un-initialized fields
	//
	if b.repo == nil {
		b.repo = repo.NewInMemoryRepo()
	}

	var err error
	if b.journal == nil {
		b.journal = journal.NewNoopJournal()
	}

	b.genBlk, err = chain2.GenesisBlock(ctx, b.repo.ChainDatastore(), b.repo.Datastore())
	if err != nil {
		return nil, err
	}

	if b.chainClock == nil {
		// get the genesis block time from the chainsubmodule
		b.chainClock = clock.NewChainClock(b.genBlk.Timestamp, b.blockTime)
	}

	// create the node
	nd := &Node{
		offlineMode: b.offlineMode,
		repo:        b.repo,
		chainClock:  b.chainClock,
	}

	// services
	nd.configModule = config2.NewConfigModule(b.repo)

	nd.blockstore, err = blockstore.NewBlockstoreSubmodule(ctx, (*builder)(b))
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.blockstore")
	}

	nd.chain, err = chain.NewChainSubmodule(ctx, (*builder)(b))
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Chain")
	}

	nd.network, err = network.NewNetworkSubmodule(ctx, nd.chain.ChainReader, nd.chain.MessageStore, (*builder)(b))
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Network")
	}

	nd.blockservice, err = dagservice.NewDagserviceSubmodule(ctx, (*builder)(b), nd.network)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.dagservice")
	}

	nd.syncer, err = syncer.NewSyncerSubmodule(ctx, (*builder)(b), nd.blockstore, nd.network, nd.chain, nd.chain.CirculatingSupplyCalculator)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Syncer")
	}

	nd.wallet, err = wallet.NewWalletSubmodule(ctx, b.repo, nd.configModule, nd.chain, b.walletPassword)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.wallet")
	}

	nd.f3, err = f3.NewF3Submodule(ctx, nd.repo, nd.chain, nd.network, nd.wallet.GetWalletSign(), nd.syncer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.f3")
	}

	nd.mpool, err = mpool.NewMpoolSubmodule(ctx, (*builder)(b), nd.network, nd.chain, nd.wallet)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.mpool")
	}

	nd.storageNetworking, err = storagenetworking.NewStorgeNetworkingSubmodule(ctx, nd.network)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.storageNetworking")
	}
	nd.mining = mining.NewMiningModule(nd.syncer.Stmgr, (*builder)(b), nd.chain, nd.blockstore, nd.syncer, *nd.wallet)

	mgrps := &paychmgr.ManagerParams{
		MPoolAPI:     nd.mpool.API(),
		ChainInfoAPI: nd.chain.API(),
		SM:           nd.syncer.Stmgr,
		WalletAPI:    nd.wallet.API(),
	}
	if nd.paychan, err = paych.NewPaychSubmodule(ctx, b.repo.PaychDatastore(), mgrps); err != nil {
		return nil, err
	}
	nd.market = market.NewMarketModule(nd.chain.API(), nd.syncer.Stmgr)

	blockDelay := b.repo.Config().NetworkParams.BlockDelay
	nd.common = common.NewCommonModule(nd.chain, nd.network, blockDelay)

	sqlitePath, err := b.repo.SqlitePath()
	if err != nil {
		return nil, err
	}
	if nd.eth, err = eth.NewEthSubModule(ctx, b.repo.Config(), nd.chain, nd.mpool, sqlitePath, nd.syncer.API()); err != nil {
		return nil, err
	}

	if nd.actorEvent, err = actorevent.NewActorEventSubModule(ctx, b.repo.Config(), nd.chain, nd.eth); err != nil {
		return nil, err
	}

	apiBuilder := NewBuilder()
	apiBuilder.NameSpace("Filecoin")

	err = apiBuilder.AddServices(nd.configModule,
		nd.blockstore,
		nd.network,
		nd.blockservice,
		nd.chain,
		nd.syncer,
		nd.wallet,
		nd.storageNetworking,
		nd.mining,
		nd.mpool,
		nd.paychan,
		nd.market,
		nd.common,
		nd.eth,
		nd.actorEvent,
		nd.f3,
	)

	if err != nil {
		return nil, errors.Wrap(err, "add service failed ")
	}

	var client *jwtclient.AuthClient
	cfg := nd.repo.Config()
	if len(cfg.API.VenusAuthURL) > 0 {
		client, err = jwtclient.NewAuthClient(cfg.API.VenusAuthURL, cfg.API.VenusAuthToken)
		if err != nil {
			return nil, fmt.Errorf("failed to create remote jwt auth client: %w", err)
		}
		nd.remoteAuth = jwtclient.WarpIJwtAuthClient(client)
	}

	var ratelimiter *ratelimit.RateLimiter
	if client != nil && cfg.RateLimitCfg.Enable {
		if ratelimiter, err = ratelimit.NewRateLimitHandler(cfg.RateLimitCfg.Endpoint,
			nil, &core.ValueFromCtx{}, jwtclient.WarpLimitFinder(client), logging.Logger("rate-limit")); err != nil {
			return nil, fmt.Errorf("request rate-limit is enabled, but create rate-limit handler failed:%w", err)
		}
		_ = logging.SetLogLevel("rate-limit", "warn")
	}

	nd.jsonRPCServiceV1 = apiBuilder.Build("v1", ratelimiter)
	nd.jsonRPCService = apiBuilder.Build("v0", ratelimiter)
	return nd, nil
}
