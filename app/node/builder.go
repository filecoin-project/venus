package node

import (
	"context"
	"time"

	"github.com/filecoin-project/venus/pkg/constants"

	"github.com/filecoin-project/venus/pkg/jwtauth"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/app/submodule/blockservice"
	"github.com/filecoin-project/venus/app/submodule/blockstore"
	"github.com/filecoin-project/venus/app/submodule/chain"
	config2 "github.com/filecoin-project/venus/app/submodule/config"
	"github.com/filecoin-project/venus/app/submodule/discovery"
	"github.com/filecoin-project/venus/app/submodule/mining"
	"github.com/filecoin-project/venus/app/submodule/mpool"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/app/submodule/storagenetworking"
	"github.com/filecoin-project/venus/app/submodule/syncer"
	"github.com/filecoin-project/venus/app/submodule/wallet"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/journal"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/specactors/policy"
	"github.com/filecoin-project/venus/pkg/util"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
	"github.com/filecoin-project/venus/pkg/version"
)

// Builder is a helper to aid in the construction of a filecoin node.
type Builder struct {
	blockTime   time.Duration
	libp2pOpts  []libp2p.Option
	offlineMode bool
	verifier    ffiwrapper.Verifier
	propDelay   time.Duration
	repo        repo.Repo
	journal     journal.Journal
	isRelay     bool
	chainClock  clock.ChainEpochClock
	genCid      cid.Cid
}

// BuilderOpt is an option for building a filecoin node.
type BuilderOpt func(*Builder) error

// offlineMode enables or disables offline mode.
func OfflineMode(offlineMode bool) BuilderOpt {
	return func(c *Builder) error {
		c.offlineMode = offlineMode
		return nil
	}
}

// IsRelay configures node to act as a libp2p relay.
func IsRelay() BuilderOpt {
	return func(c *Builder) error {
		c.isRelay = true
		return nil
	}
}

// BlockTime sets the blockTime.
func BlockTime(blockTime time.Duration) BuilderOpt {
	return func(c *Builder) error {
		c.blockTime = blockTime
		return nil
	}
}

// PropagationDelay sets the time the node needs to wait for blocks to arrive before mining.
func PropagationDelay(propDelay time.Duration) BuilderOpt {
	return func(c *Builder) error {
		c.propDelay = propDelay
		return nil
	}
}

// Libp2pOptions returns a builder option that sets up the libp2p node
func Libp2pOptions(opts ...libp2p.Option) BuilderOpt {
	return func(b *Builder) error {
		// Quietly having your options overridden leads to hair loss
		if len(b.libp2pOpts) > 0 {
			panic("Libp2pOptions can only be called once")
		}
		b.libp2pOpts = opts
		return nil
	}
}

// VerifierConfigOption returns a function that sets the verifier to use in the node consensus
func VerifierConfigOption(verifier ffiwrapper.Verifier) BuilderOpt {
	return func(c *Builder) error {
		c.verifier = verifier
		return nil
	}
}

// ChainClockConfigOption returns a function that sets the chainClock to use in the node.
func ChainClockConfigOption(clk clock.ChainEpochClock) BuilderOpt {
	return func(c *Builder) error {
		c.chainClock = clk
		return nil
	}
}

// JournalConfigOption returns a function that sets the journal to use in the node.
func JournalConfigOption(jrl journal.Journal) BuilderOpt {
	return func(c *Builder) error {
		c.journal = jrl
		return nil
	}
}

// MonkeyPatchNetworkParamsOption returns a function that sets global vars in the
// binary's specs actor dependency to change network parameters that live there
func MonkeyPatchNetworkParamsOption(params *config.NetworkParamsConfig) BuilderOpt {
	return func(c *Builder) error {
		SetNetParams(params)
		return nil
	}
}

func SetNetParams(params *config.NetworkParamsConfig) {
	if params.ConsensusMinerMinPower > 0 {
		policy.SetConsensusMinerMinPower(big.NewIntUnsigned(params.ConsensusMinerMinPower))
	}
	if len(params.ReplaceProofTypes) > 0 {
		newSupportedTypes := make([]abi.RegisteredSealProof, len(params.ReplaceProofTypes))
		for idx, proofType := range params.ReplaceProofTypes {
			newSupportedTypes[idx] = abi.RegisteredSealProof(proofType)
		}
		// Switch reference rather than mutate in place to avoid concurrent map mutation (in tests).
		policy.SetSupportedProofTypes(newSupportedTypes...)
	}

	if params.MinVerifiedDealSize > 0 {
		policy.SetMinVerifiedDealSize(abi.NewStoragePower(params.MinVerifiedDealSize))
	}

	constants.SetAddressNetwork(params.AddressNetwork)
}

// MonkeyPatchSetProofTypeOption returns a function that sets package variable
// SuppurtedProofTypes to be only the given registered proof type
func MonkeyPatchSetProofTypeOption(proofType abi.RegisteredSealProof) BuilderOpt {
	return func(c *Builder) error {
		// Switch reference rather than mutate in place to avoid concurrent map mutation (in tests).
		policy.SetSupportedProofTypes(proofType)
		return nil
	}
}

// New creates a new node.
func New(ctx context.Context, opts ...BuilderOpt) (*Node, error) {
	// initialize builder and set base values
	n := &Builder{
		offlineMode: false,
		blockTime:   clock.DefaultEpochDuration,
		propDelay:   clock.DefaultPropagationDelay,
		verifier:    ffiwrapper.ProofVerifier,
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
	b.genCid, err = readGenesisCid(b.repo.ChainDatastore())
	if err != nil {
		return nil, err
	}

	// create the node
	nd := &Node{
		offlineMode: b.offlineMode,
		repo:        b.repo,
	}
	nd.configModule = config2.NewConfigModule(b.repo)
	nd.blockstore, err = blockstore.NewBlockstoreSubmodule(ctx, b.repo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.blockstore")
	}

	nd.network, err = network.NewNetworkSubmodule(ctx, (*builder)(b), b.repo, nd.blockstore)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Network")
	}

	nd.VersionTable, err = version.ConfigureProtocolVersions(nd.network.NetworkName)
	if err != nil {
		return nil, err
	}

	nd.blockservice, err = blockservice.NewBlockserviceSubmodule(ctx, nd.blockstore, nd.network)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.blockservice")
	}

	nd.chain, err = chain.NewChainSubmodule((*builder)(b), b.repo, nd.blockstore, b.verifier)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Chain")
	}

	if b.chainClock == nil {
		// get the genesis block time from the chainsubmodule
		geneBlk, err := nd.chain.ChainReader.GetGenesisBlock(ctx)
		if err != nil {
			return nil, err
		}
		b.chainClock = clock.NewChainClock(geneBlk.Timestamp, b.blockTime, b.propDelay)
	}

	nd.chainClock = b.chainClock

	//todo chainge builder interface to read config
	nd.discovery, err = discovery.NewDiscoverySubmodule(ctx, (*builder)(b), b.repo.Config().Bootstrap, nd.network, nd.chain.ChainReader, nd.chain.MessageStore)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.discovery")
	}

	nd.syncer, err = syncer.NewSyncerSubmodule(ctx, (*builder)(b), nd.blockstore, nd.network, nd.discovery, nd.chain, b.verifier)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Syncer")
	}

	nd.wallet, err = wallet.NewWalletSubmodule(ctx, nd.configModule, b.repo, nd.chain)
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
	nd.mining = mining.NewMiningModule(b.repo, nd.chain, nd.blockstore, nd.network, nd.syncer, *nd.wallet, b.verifier)
	nd.jwtAuth, err = jwtauth.NewJwtAuth(b.repo)
	if err != nil {
		return nil, xerrors.Errorf("read or generate jwt secrect error %s", err)
	}

	apiBuilder := util.NewBuiler()
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
		nd.jwtAuth,
	)
	if err != nil {
		return nil, errors.Wrap(err, "add service failed ")
	}
	nd.jsonRPCService = apiBuilder.Build()
	return nd, nil
}

// repo returns the repo.
func (b Builder) Repo() repo.Repo {
	return b.repo
}

// Builder private method accessors for impl's

type builder Builder

func (b builder) GenesisCid() cid.Cid {
	return b.genCid
}

func (b builder) BlockTime() time.Duration {
	return b.blockTime
}

func (b builder) Repo() repo.Repo {
	return b.repo
}

func (b builder) IsRelay() bool {
	return b.isRelay
}

func (b builder) ChainClock() clock.ChainEpochClock {
	return b.chainClock
}

func (b builder) Journal() journal.Journal {
	return b.journal
}

func (b builder) Libp2pOpts() []libp2p.Option {
	return b.libp2pOpts
}

func (b builder) OfflineMode() bool {
	return b.offlineMode
}
