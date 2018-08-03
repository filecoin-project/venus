package chain

import (
	"context"

	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	"gx/ipfs/QmdbxjQWogRCHRaxhhGnYdT1oQJzL9GdqSKzCdqWr85AP2/pubsub"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

// NewHeadTopic is the topic used to publish new heads.
const NewHeadTopic = "new-head"

// ReadStore is the read-only subset of the chain.Store interface.  Most callers
// of the store should use this interface.  Some of chain.Store's read methods
// are not on this interface because they are only used by parts of the system
// that require a Store interface.
type ReadStore interface {
	// Load loads the chain from disk.
	Load(ctx context.Context) error
	// Stop stops all activities and cleans up.
	Stop()

	// GetTipSet retrieves the tipindex value (tipset, state) at the
	// provided tipset key if in the store and an error if it does not
	// exist.
	GetTipSetAndState(ctx context.Context, tsKey string) (*TipSetAndState, error)
	// GetBlock gets a block by cid.
	GetBlock(ctx context.Context, id *cid.Cid) (*types.Block, error)

	HeadEvents() *pubsub.PubSub
	// Head returns the head of the chain tracked by the store.
	Head() consensus.TipSet
	// LatestState returns the latest state of the head
	LatestState(ctx context.Context) (state.Tree, error)

	BlockHistory(ctx context.Context) <-chan interface{}
	GenesisCid() *cid.Cid
}

// Store wraps the on-disk storage of a valid blockchain.  Callers can get and
// set blocks and tipsets into the store.  Callers can set and get the head of
// the chain, i.e. the heaviest known tipset.  Methods exist for iterating
// over the ancestors of a block in the store and listening on events sent
// by the Store.
// The only part of the system with the privilege to use this interface is
// currently the chain.Syncer.  All other callers should use a chain.ReadStore
// instead.
type Store interface {
	ReadStore

	// PutTipSet adds a tipset to the store.  This persists blocks to disk and
	// updates the tips index.
	PutTipSetAndState(ctx context.Context, tsas *TipSetAndState) error
	// HasTipSet indicates whether the tipset is in the store.
	HasTipSetAndState(ctx context.Context, tsKey string) bool
	// GetTipSetsByParents returns all tipsets with the given parent set.
	GetTipSetAndStatesByParents(ctx context.Context, pTsKey string) ([]*TipSetAndState, error)
	// HasTipSetsWithParents indicates whether tipsets with these parents are in the store.
	HasTipSetAndStatesWithParents(ctx context.Context, pTsKey string) bool

	// GetBlocks gets several blocks by cid. In the future there is caching here
	GetBlocks(ctx context.Context, ids types.SortedCidSet) ([]*types.Block, error)
	// HasAllBlocks indicates whether the blocks are in the store.
	HasAllBlocks(ctx context.Context, cs []*cid.Cid) bool
	HasBlock(ctx context.Context, c *cid.Cid) bool

	// SetHead sets the internally tracked  head to the provided tipset.
	SetHead(ctx context.Context, s consensus.TipSet) error
}
