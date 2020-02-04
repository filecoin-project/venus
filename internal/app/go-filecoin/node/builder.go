package node

import (
	"context"
	"time"

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
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/strgdls"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/journal"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/version"
)

// Builder is a helper to aid in the construction of a filecoin node.
type Builder struct {
	blockTime   time.Duration
	libp2pOpts  []libp2p.Option
	offlineMode bool
	verifier    verification.Verifier
	repo        repo.Repo
	journal     journal.Journal
	isRelay     bool
	chainClock  clock.ChainEpochClock
	genCid      cid.Cid
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

// Libp2pOptions returns a node config option that sets up the libp2p node
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
func VerifierConfigOption(verifier verification.Verifier) BuilderOpt {
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

// New creates a new node.
func New(ctx context.Context, opts ...BuilderOpt) (*Node, error) {
	// initialize builder and set base values
	n := &Builder{
		offlineMode: false,
		blockTime:   clock.DefaultEpochDuration,
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
	if b.journal == nil {
		b.journal = journal.NewNoopJournal()
	}

	var err error

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

	nd.chain, err = submodule.NewChainSubmodule(ctx, (*builder)(b), b.repo, &nd.Blockstore)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Chain")
	}

	if b.chainClock == nil {
		// get the genesis block time from the chainsubmodule
		geneBlk, err := nd.chain.ChainReader.GetGenesisBlock(ctx)
		if err != nil {
			return nil, err
		}
		b.chainClock = clock.NewChainClock(uint64(geneBlk.Timestamp), b.blockTime)
	}
	nd.ChainClock = b.chainClock

	nd.syncer, err = submodule.NewSyncerSubmodule(ctx, (*builder)(b), b.repo, &nd.Blockstore, &nd.network, &nd.Discovery, &nd.chain, nd.ProofVerification.ProofVerifier)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Syncer")
	}

	nd.Wallet, err = submodule.NewWalletSubmodule(ctx, b.repo)
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

	nd.BlockMining, err = submodule.NewBlockMiningSubmodule(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.BlockMining")
	}

	nd.ProofVerification = submodule.NewProofVerificationSubmodule()

	nd.StorageProtocol, err = submodule.NewStorageProtocolSubmodule(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.StorageProtocol")
	}

	nd.RetrievalProtocol, err = submodule.NewRetrievalProtocolSubmodule(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.RetrievalProtocol")
	}

	nd.PorcelainAPI = porcelain.New(plumbing.New(&plumbing.APIDeps{
		Chain:        nd.chain.State,
		Sync:         cst.NewChainSyncProvider(nd.syncer.ChainSyncManager),
		Config:       cfg.NewConfig(b.repo),
		DAG:          dag.NewDAG(merkledag.NewDAGService(nd.Blockservice.Blockservice)),
		Deals:        strgdls.New(b.repo.DealsDatastore()),
		Expected:     nd.syncer.Consensus,
		MsgPool:      nd.Messaging.MsgPool,
		MsgPreviewer: msg.NewPreviewer(nd.chain.ChainReader, nd.Blockstore.CborStore, nd.Blockstore.Blockstore, nd.chain.Processor),
		ActState:     nd.chain.ActorState,
		MsgWaiter:    msg.NewWaiter(nd.chain.ChainReader, nd.chain.MessageStore, nd.Blockstore.Blockstore, nd.Blockstore.CborStore),
		Network:      nd.network.Network,
		Outbox:       nd.Messaging.Outbox,
		PieceManager: nd.PieceManager,
		Wallet:       nd.Wallet.Wallet,
	}))

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
