package node

import (
	"context"
	"time"

	chain2 "github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/jwtauth"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper/impl"
	"github.com/ipfs-force-community/metrics/ratelimit"
	logging "github.com/ipfs/go-log"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/app/submodule/blockstore"
	"github.com/filecoin-project/venus/app/submodule/chain"
	config2 "github.com/filecoin-project/venus/app/submodule/config"
	"github.com/filecoin-project/venus/app/submodule/dagservice"
	"github.com/filecoin-project/venus/app/submodule/discovery"
	"github.com/filecoin-project/venus/app/submodule/market"
	"github.com/filecoin-project/venus/app/submodule/mining"
	"github.com/filecoin-project/venus/app/submodule/mpool"
	"github.com/filecoin-project/venus/app/submodule/multisig"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/app/submodule/paych"
	"github.com/filecoin-project/venus/app/submodule/storagenetworking"
	"github.com/filecoin-project/venus/app/submodule/syncer"
	"github.com/filecoin-project/venus/app/submodule/wallet"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/journal"
	"github.com/filecoin-project/venus/pkg/paychmgr"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/statemanger"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
	"github.com/libp2p/go-libp2p"
	"github.com/pkg/errors"
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

	// fetch genesis block id
	b.genBlk, err = readGenesisCid(b.repo.ChainDatastore(), b.repo.Datastore())
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
	}

	//modules
	nd.circulatiingSupplyCalculator = chain2.NewCirculatingSupplyCalculator(b.repo.Datastore(), b.genBlk.ParentStateRoot, b.repo.Config().NetworkParams.ForkUpgradeParam)

	//services
	nd.configModule = config2.NewConfigModule(b.repo)

	nd.blockstore, err = blockstore.NewBlockstoreSubmodule(ctx, (*builder)(b))
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.blockstore")
	}

	nd.network, err = network.NewNetworkSubmodule(ctx, (*builder)(b))
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Network")
	}

	nd.blockservice, err = dagservice.NewDagserviceSubmodule(ctx, (*builder)(b), nd.network)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.dagservice")
	}

	nd.chain, err = chain.NewChainSubmodule(ctx, (*builder)(b), nd.circulatiingSupplyCalculator)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Chain")
	}

	nd.chainClock = b.chainClock

	// todo change builder interface to read config
	nd.discovery, err = discovery.NewDiscoverySubmodule(ctx, (*builder)(b), nd.network, nd.chain.ChainReader, nd.chain.MessageStore)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.discovery")
	}

	nd.syncer, err = syncer.NewSyncerSubmodule(ctx, (*builder)(b), nd.blockstore, nd.network, nd.discovery, nd.chain, nd.circulatiingSupplyCalculator)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Syncer")
	}

	nd.wallet, err = wallet.NewWalletSubmodule(ctx, b.repo, nd.configModule, nd.chain, b.walletPassword)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.wallet")
	}

	nd.mpool, err = mpool.NewMpoolSubmodule((*builder)(b), nd.network, nd.chain, nd.syncer, nd.wallet)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.mpool")
	}

	nd.storageNetworking, err = storagenetworking.NewStorgeNetworkingSubmodule(ctx, nd.network)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.storageNetworking")
	}
	nd.mining = mining.NewMiningModule((*builder)(b), nd.chain, nd.blockstore, nd.network, nd.syncer, *nd.wallet)

	nd.multiSig = multisig.NewMultiSigSubmodule(nd.chain.API(), nd.mpool.API(), nd.chain.ChainReader)

	stmgr := statemanger.NewStateMangerAPI(nd.chain.ChainReader, nd.syncer.Consensus)
	mgrps := &paychmgr.ManagerParams{
		MPoolAPI:     nd.mpool.API(),
		ChainInfoAPI: nd.chain.API(),
		SM:           stmgr,
		WalletAPI:    nd.wallet.API(),
	}
	if nd.paychan, err = paych.NewPaychSubmodule(ctx, b.repo.PaychDatastore(), mgrps); err != nil {
		return nil, err
	}
	nd.market = market.NewMarketModule(nd.chain.API(), stmgr)

	apiBuilder := NewBuilder()
	apiBuilder.NameSpace("Filecoin")

	err = apiBuilder.AddServices(nd.configModule,
		nd.blockstore,
		nd.network,
		nd.blockservice,
		nd.discovery,
		nd.chain,
		nd.syncer,
		nd.wallet,
		nd.storageNetworking,
		nd.mining,
		nd.mpool,
		nd.paychan,
		nd.market)

	if err != nil {
		return nil, errors.Wrap(err, "add service failed ")
	}

	cfg := nd.repo.Config()
	if len(cfg.API.VenusAuthURL) > 0 {
		nd.remoteAuth = jwtauth.NewRemoteAuth(cfg.API.VenusAuthURL)
	}

	var ratelimiter *ratelimit.RateLimiter
	if nd.remoteAuth != nil && cfg.RateLimitCfg.Enable {
		if ratelimiter, err = ratelimit.NewRateLimitHandler(cfg.RateLimitCfg.Endpoint,
			nil, &jwtauth.ValueFromCtx{}, nd.remoteAuth, logging.Logger("rate-limit")); err != nil {
			return nil, xerrors.Errorf("request rate-limit is enabled, but create rate-limit handler failed:%w", err)
		}
		_ = logging.SetLogLevel("rate-limit", "warn")
	}

	nd.jsonRPCServiceV1 = apiBuilder.Build("v1", ratelimiter)
	nd.jsonRPCService = apiBuilder.Build("v0", ratelimiter)
	return nd, nil
}
