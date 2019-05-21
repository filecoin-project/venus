package chain

import (
	"context"

	"github.com/cskr/pubsub"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"

	"github.com/filecoin-project/go-filecoin/types"
)

// NewHeadTopic is the topic used to publish new heads.
const NewHeadTopic = "new-head"

// GenesisKey is the key at which the genesis Cid is written in the datastore.
var GenesisKey = datastore.NewKey("/consensus/genesisCid")

// ReadStore is the read-only subset of the chain.Store interface.  Most callers
// of the store should use this interface.  Some of chain.Store's read methods
// are not on this interface because they are only used by parts of the system
// that require a Store interface.
type ReadStore interface {
	// Load loads the chain from disk.
	Load(ctx context.Context) error
	// Stop stops all activities and cleans up.
	Stop()

	// GetTipSet retrieves the tipset at the
	// provided tipset key if in the store and an error if it does not
	// exist.
	GetTipSet(tsKey types.SortedCidSet) (*types.TipSet, error)

	// GetTipSet retrieves the state at the
	// provided tipset key if in the store and an error if it does not
	// exist.
	GetTipSetStateRoot(tsKey types.SortedCidSet) (cid.Cid, error)

	// GetBlock gets a block by cid.
	GetBlock(ctx context.Context, id cid.Cid) (*types.Block, error)

	HeadEvents() *pubsub.PubSub
	// GetHead returns the head of the chain tracked by the store.
	GetHead() types.SortedCidSet

	GenesisCid() cid.Cid

	//returns the chain height of the head tipset
	BlockHeight() (uint64, error)
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
	// GetTipSetsByParentsAndHeight returns all tipsets with the given parent set and the given height
	GetTipSetAndStatesByParentsAndHeight(pTsKey string, h uint64) ([]*TipSetAndState, error)
	// HasTipSetsWithParentsAndHeight indicates whether tipsets with these parents and this height are in the store.
	HasTipSetAndStatesWithParentsAndHeight(pTsKey string, h uint64) bool

	// GetBlocks gets several blocks by cid. In the future there is caching here
	GetBlocks(ctx context.Context, cids types.SortedCidSet) ([]*types.Block, error)
	// HasAllBlocks indicates whether the blocks are in the store.
	HasAllBlocks(ctx context.Context, cs []cid.Cid) bool
	HasBlock(ctx context.Context, c cid.Cid) bool

	// SetHead sets the internally tracked  head to the provided tipset.
	SetHead(ctx context.Context, s types.TipSet) error
}
