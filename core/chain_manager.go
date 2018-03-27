package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"
	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"
	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/QmdbxjQWogRCHRaxhhGnYdT1oQJzL9GdqSKzCdqWr85AP2/pubsub"
	hamt "gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/types"
)

var log = logging.Logger("chain")

var (
	// ErrStateRootMismatch is returned when the computed state root doesn't match the expected result.
	ErrStateRootMismatch = errors.New("blocks state root does not match computed result")
	// ErrInvalidBase is returned when the chain doesn't connect back to a known good block.
	ErrInvalidBase = errors.New("block does not connect to a known good chain")
	// ErrDifferentGenesis is returned when processing a chain with a different genesis block.
	ErrDifferentGenesis = fmt.Errorf("chain had different genesis")
)

var bestBlockKey = datastore.NewKey("/chain/bestBlock")

// BlockTopic is the topic used to publish new best blocks.
const BlockTopic = "blocks"

// BlockProcessResult signifies the outcome of processing a given block.
type BlockProcessResult int

const (
	// Unknown implies there was an error that made it impossible to process the block.
	Unknown = BlockProcessResult(iota)

	// ChainAccepted implies the chain was valid, and is now our current best
	// chain.
	ChainAccepted

	// ChainValid implies the chain was valid, but not better than our current
	// best chain.
	ChainValid

	// InvalidBase implies the chain does not connect back to any previously
	// known good block.
	InvalidBase
)

// ChainManager manages the current state of the chain and handles validating
// and applying updates.
// Safe for concurrent access
type ChainManager struct {
	// bestBlock is the head block of the best known chain
	bestBlock struct {
		sync.Mutex
		blk *types.Block
	}

	processor Processor

	// genesisCid holds the cid of the chains genesis block for later access
	genesisCid *cid.Cid

	// knownGoodBlocks is a cache of 'good blocks'. It is a cache to prevent us
	// from having to rescan parts of the blockchain when determining the
	// validity of a given chain.
	// In the future we will need a more sophisticated mechanism here.
	// TODO: this should probably be an LRU, needs more consideration.
	// For example, the genesis block should always be considered a "good" block.
	knownGoodBlocks SyncCidSet

	cstore *hamt.CborIpldStore

	ds datastore.Datastore

	// BestBlockPubSub is a pubsub channel that publishes all best blocks.
	// We operate under the assumption that blocks published to this channel
	// will always be queued and delivered to subscribers in the order discovered.
	BestBlockPubSub *pubsub.PubSub

	FetchBlock func(context.Context, *cid.Cid) (*types.Block, error)
}

// NewChainManager creates a new filecoin chain manager.
func NewChainManager(ds datastore.Datastore, cs *hamt.CborIpldStore) *ChainManager {
	cm := &ChainManager{
		cstore:          cs,
		ds:              ds,
		processor:       ProcessBlock,
		BestBlockPubSub: pubsub.New(128),
	}
	cm.FetchBlock = cm.fetchBlock
	cm.knownGoodBlocks.set = cid.NewSet()

	return cm
}

// Genesis creates a new genesis block and sets it as the the best known block.
func (s *ChainManager) Genesis(ctx context.Context, gen GenesisInitFunc) error {
	genesis, err := gen(s.cstore)
	if err != nil {
		return err
	}

	s.genesisCid = genesis.Cid()

	s.bestBlock.Lock()
	defer s.bestBlock.Unlock()
	return s.setBestBlock(ctx, genesis)
}

// setBestBlock sets the best block. Should be used to either to set the
// genesis block, or to manually set the current selected chain for testing.
// CALLER MUST HOLD THE bestBlock LOCK.
func (s *ChainManager) setBestBlock(ctx context.Context, b *types.Block) error {
	_, err := s.cstore.Put(ctx, b)
	if err != nil {
		return errors.Wrap(err, "failed to put block to disk")
	}
	s.bestBlock.blk = b // TODO: make a copy?
	s.knownGoodBlocks.Add(b.Cid())

	if err := putCid(s.ds, bestBlockKey, b.Cid()); err != nil {
		return errors.Wrap(err, "failed to write bestBlockCid to datastore")
	}

	s.BestBlockPubSub.Pub(b, BlockTopic)

	return nil
}

func putCid(ds datastore.Datastore, k datastore.Key, c *cid.Cid) error {
	return ds.Put(k, c.Bytes())
}

// Load reads the best block cid from disk and reparses the chain backwards from there.
func (s *ChainManager) Load() error {
	bbc, err := s.readBestBlockCid()
	if err != nil {
		return err
	}

	// TODO: 'read only from local disk' method here.
	// actually, i think that the chainmanager should only ever fetch from
	// the local disk unless we're syncing. Its something that needs more
	// thought at least.
	blk, err := s.fetchBlock(context.TODO(), bbc)
	if err != nil {
		return errors.Wrap(err, "scanning blockchain failed")
	}

	headBlk := blk

	for blk.Parent != nil {
		// TODO: hrm... adding these in reverse order
		s.knownGoodBlocks.Add(blk.Cid())
		next, err := s.fetchBlock(context.TODO(), blk.Parent)
		if err != nil {
			return errors.Wrap(err, "scanning blockchain failed")
		}

		blk = next
	}

	// TODO: probably want to load the expected genesis block and assert it here?
	s.genesisCid = blk.Cid()
	s.bestBlock.blk = headBlk

	return nil
}

func (s *ChainManager) readBestBlockCid() (*cid.Cid, error) {
	bbi, err := s.ds.Get(bestBlockKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read best block cid")
	}

	bb, ok := bbi.([]byte)
	if !ok {
		return nil, fmt.Errorf("stored bestBlockCid was not []byte")
	}

	bestBlockCid, err := cid.Cast(bb)
	if err != nil {
		return nil, errors.Wrap(err, "casting stored bestBlockCid failed")
	}

	return bestBlockCid, nil
}

// GetGenesisCid returns the cid of the current genesis block.
func (s *ChainManager) GetGenesisCid() *cid.Cid {
	return s.genesisCid
}

// GetBestBlock returns the head of our currently selected 'best' chain.
func (s *ChainManager) GetBestBlock() *types.Block {
	s.bestBlock.Lock()
	defer s.bestBlock.Unlock()
	return s.bestBlock.blk // TODO: return immutable copy?
}

func (s *ChainManager) maybeAcceptBlock(ctx context.Context, blk *types.Block) (BlockProcessResult, error) {
	// We have to hold the lock at this level to avoid TOCTOU problems
	// with the new best block.
	s.bestBlock.Lock()
	defer s.bestBlock.Unlock()
	if blk.Score() <= s.bestBlock.blk.Score() {
		return ChainValid, nil
	}

	return s.acceptNewBestBlock(ctx, blk)
}

// ProcessNewBlock sends a new block to the chain manager. If the block is
// better than our current best, it is accepted as our new best block.
// Otherwise an error is returned explaining why it was not accepted
func (s *ChainManager) ProcessNewBlock(ctx context.Context, blk *types.Block) (BlockProcessResult, error) {
	log.Infof("processing block [s=%d, h=%s]", blk.Score(), blk.Cid())

	switch err := s.validateBlock(ctx, blk); err {
	default:
		return Unknown, errors.Wrap(err, "validate block failed")
	case ErrInvalidBase:
		return InvalidBase, ErrInvalidBase
	case nil:
		return s.maybeAcceptBlock(ctx, blk)
	}
}

// acceptNewBestBlock sets the given block as our current 'best chain' block.
// CALLER MUST HOLD THE bestBlock LOCK.
func (s *ChainManager) acceptNewBestBlock(ctx context.Context, blk *types.Block) (BlockProcessResult, error) {
	if err := s.setBestBlock(ctx, blk); err != nil {
		return Unknown, err
	}
	log.Infof("accepted new block, [s=%d, h=%s]", blk.Score(), blk.Cid())
	return ChainAccepted, nil
}

// fetchBlock gets the requested block, either from disk or from the network.
func (s *ChainManager) fetchBlock(ctx context.Context, c *cid.Cid) (*types.Block, error) {
	log.Infof("fetching block, [%s]", c.String())

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	var blk types.Block
	if err := s.cstore.Get(ctx, c, &blk); err != nil {
		return nil, err
	}

	return &blk, nil
}

// checkBlockValid verifies that this block, on its own, is structurally and
// cryptographically valid. This means checking that all of its fields are
// properly filled out and its signatures are correct. Checking the validity of
// state changes must be done separately and only once the state of the
// previous block has been validated.
func (s *ChainManager) validateBlockStructure(ctx context.Context, b *types.Block) error {
	// TODO: validate signatures on messages
	if b.StateRoot == nil {
		return fmt.Errorf("block has nil StateRoot")
	}

	return nil
}

// TODO: this method really needs to be thought through carefully. Probably one
// of the most complicated bits of the system
func (s *ChainManager) validateBlock(ctx context.Context, b *types.Block) error {
	if err := s.validateBlockStructure(ctx, b); err != nil {
		return errors.Wrap(err, "check block valid failed")
	}

	if _, err := s.cstore.Put(ctx, b); err != nil {
		return errors.Wrap(err, "failed to store block")
	}

	baseBlk, chain, err := s.findKnownAncestor(ctx, b)
	if err != nil {
		return err
	}

	st, err := types.LoadStateTree(ctx, s.cstore, baseBlk.StateRoot)
	if err != nil {
		return err
	}

	for i := len(chain) - 1; i >= 0; i-- {
		cur := chain[i]
		receipts, err := s.processor(ctx, cur, st)
		if err != nil {
			return err
		}

		// TODO: more sophisticated comparison
		if len(receipts) != len(cur.MessageReceipts) {
			return fmt.Errorf("found invalid message receipts: %v %v", receipts, cur.MessageReceipts)
		}

		// TODO: check that state transitions are valid once we have them
		s.knownGoodBlocks.Add(cur.Cid())
	}

	outCid, err := st.Flush(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to flush tree after applying state transitions")
	}

	if !outCid.Equals(b.StateRoot) {
		return ErrStateRootMismatch
	}

	return nil
}

// findKnownAcestor walks backwards from the given block until it finds a block
// that we know to be good. It then returns that known block, and the blocks
// that form the chain back to it.
func (s *ChainManager) findKnownAncestor(ctx context.Context, tip *types.Block) (*types.Block, []*types.Block, error) {
	// TODO: should be some sort of limit here
	// Some implementations limit the length of a chain that can be swapped.
	// Historically, bitcoin does not, this is purely for religious and
	// idealogical reasons. In reality, if a weeks worth of blocks is about to
	// be reverted, the system should opt to halt, not just happily switch over
	// to an entirely different chain.
	var validating []*types.Block
	baseBlk := tip
	for !s.isKnownGoodBlock(baseBlk.Cid()) {
		if baseBlk.Parent == nil {
			return nil, nil, ErrInvalidBase
		}

		validating = append(validating, baseBlk)

		next, err := s.FetchBlock(ctx, baseBlk.Parent)
		if err != nil {
			return nil, nil, errors.Wrap(err, "fetch block failed")
		}

		if err := s.validateBlockStructure(ctx, next); err != nil {
			return nil, nil, err
		}

		baseBlk = next
	}

	return baseBlk, validating, nil
}

func (s *ChainManager) isKnownGoodBlock(bc *cid.Cid) bool {
	return bc.Equals(s.genesisCid) || s.knownGoodBlocks.Has(bc)
}

// InformNewBlock informs the chainmanager that we learned about a potentially
// new block from the given peer. Currently, it just fetches that block and
// passes it to the block processor (which fetches the rest of the chain on
// demand). In the (near) future we will want a better protocol for
// synchronizing the blockchain and downloading it efficiently.
// TODO: sync logic should be decoupled and off in a separate worker. This
// method should not block
func (s *ChainManager) InformNewBlock(from peer.ID, c *cid.Cid, h uint64) {
	b := s.GetBestBlock()
	if b.Height >= h {
		return
	}

	// Naive sync.
	// TODO: more dedicated sync protocols, like "getBlockHashes(range)"
	ctx := context.TODO()
	blk, err := s.FetchBlock(ctx, c)
	if err != nil {
		log.Error("failed to fetch block: ", err)
		return
	}

	_, err = s.ProcessNewBlock(ctx, blk)
	if err != nil {
		log.Error("processing new block: ", err)
		return
	}
}

// Stop stops all activities and cleans up.
func (s *ChainManager) Stop() {
	s.BestBlockPubSub.Shutdown()
}

// ChainManagerForTest provides backdoor access to internal fields to make
// testing easier. You are a bad person if you use this outside of a test.
type ChainManagerForTest = ChainManager

// SetBestBlockForTest enables setting the best block directly. Don't
// use this outside of a testing context.
func (s *ChainManagerForTest) SetBestBlockForTest(ctx context.Context, b *types.Block) error {
	return s.setBestBlock(ctx, b)
}

// BlockHistory returns a channel of block pointers (or errors), starting with the current best block
// followed by each subsequent parent and ending with the genesis block, after which the channel
// is closed. If an error is encountered while fetching a block, the error is sent, and the channel is closed.
func (s *ChainManager) BlockHistory(ctx context.Context) <-chan interface{} {
	out := make(chan interface{})
	blk := s.GetBestBlock()

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case out <- blk:
				if blk.Parent == nil {
					return
				}
				var err error
				blk, err = s.FetchBlock(ctx, blk.Parent)
				if err != nil {
					log.Errorf("failed to fetch block: %s", err)
					out <- err
					return
				}
			}
		}
	}()
	return out
}

// WaitForMessage searches for a message with Cid, msgCid, then passes it, along with the containing Block and any
// MessageRecipt, to the supplied callback, cb. If an error is encountered, it is returned. Note that it is logically
// possible that an error is returned and the success callback is called. In that case, the error can be safely ignored.
// TODO: This implementation will become prohibitively expensive since it involves traversing the entire blockchain.
//       We should replace with an index later.
func (s *ChainManager) WaitForMessage(ctx context.Context, msgCid *cid.Cid, cb func(*types.Block, *types.Message,
	*types.MessageReceipt) error) (retErr error) {
	// Ch will contain a stream of blocks to check for message (or errors).
	// Blocks are either new best blocks, or next oldest historical blocks.
	ch := make(chan (interface{}))

	// New blocks
	newBlockCh := s.BestBlockPubSub.Sub(BlockTopic)
	defer s.BestBlockPubSub.Unsub(newBlockCh, BlockTopic)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Historical blocks
	historyCh := s.BlockHistory(ctx)

	// Merge historical and new block channels.
	go func() {
		// TODO: accommodate a new chain being added, as opposed to just a single block.
		for raw := range newBlockCh {
			ch <- raw
		}
	}()
	go func() {
		for raw := range historyCh {
			ch <- raw
		}
	}()

	for raw := range ch {
		switch b := raw.(type) {
		case error:
			log.Errorf("chainManager.WaitForMessage: %s", b)
			return b
		case *types.Block:
			for i, msg := range b.Messages {
				c, err := msg.Cid()
				if err != nil {
					log.Errorf("chainManager.WaitForMessage: %s", err)
					return err
				}
				if c.Equals(msgCid) {
					var rcpt *types.MessageReceipt
					if i < len(b.MessageReceipts) {
						rcpt = b.MessageReceipts[i]
					}
					return cb(b, msg, rcpt)
				}
			}
		}
	}

	return retErr
}
