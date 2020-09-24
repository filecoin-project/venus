package node

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/vendors/sector-storage/ffiwrapper"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-merkledag"
	"github.com/libp2p/go-libp2p"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/internal/submodule"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cfg"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/dag"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
	"github.com/filecoin-project/go-filecoin/internal/pkg/journal"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	drandapi "github.com/filecoin-project/go-filecoin/internal/pkg/protocol/drand"
	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/version"
)

// Builder is a helper to aid in the construction of a filecoin node.
type Builder struct {
	blockTime   time.Duration
	libp2pOpts  []libp2p.Option
	offlineMode bool
	verifier    ffiwrapper.Verifier
	postGen     postgenerator.PoStGenerator
	propDelay   time.Duration
	repo        repo.Repo
	journal     journal.Journal
	isRelay     bool
	chainClock  clock.ChainEpochClock
	genCid      cid.Cid
	drand       drand.Schedule
}

// BuilderOpt is an option for building a filecoin node.
type BuilderOpt func(*Builder) error

// OfflineMode enables or disables offline mode.
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

// PoStGeneratorOption returns a builder option that sets the post generator to
// use during block generation
func PoStGeneratorOption(generator postgenerator.PoStGenerator) BuilderOpt {
	return func(b *Builder) error {
		b.postGen = generator
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

// DrandConfigOption returns a function that sets the node's drand interface
func DrandConfigOption(d drand.Schedule) BuilderOpt {
	return func(c *Builder) error {
		c.drand = d
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
		if params.ConsensusMinerMinPower > 0 {
			power.ConsensusMinerMinPower = big.NewIntUnsigned(params.ConsensusMinerMinPower)
		}
		if len(params.ReplaceProofTypes) > 0 {
			newSupportedTypes := make(map[abi.RegisteredSealProof]struct{})
			for _, proofType := range params.ReplaceProofTypes {
				newSupportedTypes[abi.RegisteredSealProof(proofType)] = struct{}{}
			}
			// Switch reference rather than mutate in place to avoid concurrent map mutation (in tests).
			miner.SupportedProofTypes = newSupportedTypes
		}
		return nil
	}
}

// MonkeyPatchSetProofTypeOption returns a function that sets package variable
// SuppurtedProofTypes to be only the given registered proof type
func MonkeyPatchSetProofTypeOption(proofType abi.RegisteredSealProof) BuilderOpt {
	return func(c *Builder) error {
		// Switch reference rather than mutate in place to avoid concurrent map mutation (in tests).
		miner.SupportedProofTypes = map[abi.RegisteredSealProof]struct{}{proofType: {}}
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
	b.genCid, err = readGenesisCid(b.repo.Datastore())
	if err != nil {
		return nil, err
	}

	// create the node
	nd := &Node{
		OfflineMode: b.offlineMode,
		Repo:        b.repo,
	}

	nd.Blockstore, err = submodule.NewBlockstoreSubmodule(ctx, b.repo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Blockstore")
	}

	nd.network, err = submodule.NewNetworkSubmodule(ctx, (*builder)(b), b.repo, &nd.Blockstore)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Network")
	}

	nd.Discovery, err = submodule.NewDiscoverySubmodule(ctx, (*builder)(b), b.repo.Config().Bootstrap, &nd.network)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Discovery")
	}

	nd.VersionTable, err = version.ConfigureProtocolVersions(nd.network.NetworkName)
	if err != nil {
		return nil, err
	}

	nd.Blockservice, err = submodule.NewBlockserviceSubmodule(ctx, &nd.Blockstore, &nd.network)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Blockservice")
	}

	nd.ProofVerification = submodule.NewProofVerificationSubmodule(b.verifier)

	nd.chain, err = submodule.NewChainSubmodule((*builder)(b), b.repo, &nd.Blockstore, &nd.ProofVerification, b.drand)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Chain")
	}
	if b.drand == nil {
		genBlk, err := nd.chain.ChainReader.GetGenesisBlock(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to construct drand grpc")
		}
		dGRPC, err := DefaultDrandIfaceFromConfig(b.repo.Config(), genBlk.Timestamp)
		if err != nil {
			return nil, err
		}
		b.drand = dGRPC
	}

	if b.chainClock == nil {
		// get the genesis block time from the chainsubmodule
		geneBlk, err := nd.chain.ChainReader.GetGenesisBlock(ctx)
		if err != nil {
			return nil, err
		}
		b.chainClock = clock.NewChainClock(geneBlk.Timestamp, b.blockTime, b.propDelay)
	}
	nd.ChainClock = b.chainClock

	nd.syncer, err = submodule.NewSyncerSubmodule(ctx, (*builder)(b), &nd.Blockstore, &nd.network, &nd.Discovery, &nd.chain, nd.ProofVerification.ProofVerifier)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Syncer")
	}

	nd.Wallet, err = submodule.NewWalletSubmodule(ctx, b.repo, &nd.chain)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Wallet")
	}

	nd.Messaging, err = submodule.NewMessagingSubmodule(ctx, (*builder)(b), b.repo, &nd.network, &nd.chain, &nd.Wallet)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Messaging")
	}

	nd.StorageNetworking, err = submodule.NewStorgeNetworkingSubmodule(ctx, &nd.network)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.StorageNetworking")
	}

	nd.BlockMining, err = submodule.NewBlockMiningSubmodule(ctx, b.postGen)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.BlockMining")
	}

	waiter := msg.NewWaiter(nd.chain.ChainReader, nd.chain.MessageStore, nd.Blockstore.Blockstore, nd.Blockstore.CborStore)

	nd.PorcelainAPI = porcelain.New(plumbing.New(&plumbing.APIDeps{
		Chain:        nd.chain.State,
		Sync:         cst.NewChainSyncProvider(nd.syncer.ChainSyncManager),
		Config:       cfg.NewConfig(b.repo),
		DAG:          dag.NewDAG(merkledag.NewDAGService(nd.Blockservice.Blockservice)),
		Expected:     nd.syncer.Consensus,
		MsgPool:      nd.Messaging.MsgPool,
		MsgPreviewer: msg.NewPreviewer(nd.chain.ChainReader, nd.Blockstore.CborStore, nd.Blockstore.Blockstore, nd.chain.Processor),
		MsgWaiter:    waiter,
		Network:      nd.network.Network,
		Outbox:       nd.Messaging.Outbox,
		PieceManager: nd.PieceManager,
		Wallet:       nd.Wallet.Wallet,
	}))

	nd.StorageProtocol, err = submodule.NewStorageProtocolSubmodule(
		ctx,
		nd.PorcelainAPI.WalletDefaultAddress,
		&nd.chain,
		&nd.Messaging,
		waiter,
		nd.Wallet.Signer,
		nd.Host(),
		nd.Repo.Datastore(),
		nd.Blockstore.Blockstore,
		nd.Repo.MultiStore(),
		nd.network.DataTransfer,
		state.NewViewer(nd.Blockstore.CborStore),
	)
	if err != nil {
		return nil, err
	}

	nd.StorageAPI = storage.NewAPI(nd.StorageProtocol)
	nd.DrandAPI = drandapi.New(b.drand, nd.PorcelainAPI)

	return nd, nil
}

// Repo returns the repo.
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

func (b builder) Drand() drand.Schedule {
	return b.drand
}

func DefaultDrandIfaceFromConfig(cfg *config.Config, fcGenTS uint64) (drand.Schedule, error) {
	return drand.RandomSchedule(time.Unix(0, 0))
}
