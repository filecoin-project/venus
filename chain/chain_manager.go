package chain

import (
	"context"
	"fmt"
	"time"

	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	hamt "gx/ipfs/QmSwABWvsucRwH7XVDbTE3aJgiVdtJUfeWjY8oejB4RmAA/go-hamt-ipld"
	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	types "github.com/filecoin-project/go-filecoin/types"
)

var log = logging.Logger("chain")

// ChainManager manages the current state of the chain and handles validating
// and applying updates.
// currently not safe for concurrent access
// TODO: make it safe for concurrent access
type ChainManager struct {
	// TODO: need some sync stuff here. Some of these fields get access in a
	// racy way right now. These fields need to be safe for concurrent access.
	BestBlock *types.Block // needs sync

	// KnownGoodBlocks is the set of 'good blocks'. It is a cache to prevent us
	// from having to rescan parts of the blockchain when determining the
	// validity of a given chain
	// In the future we will need a more sophisticated mechanism here.
	// TODO: this should probably be an LRU
	KnownGoodBlocks *cid.Set // needs sync

	cstore *hamt.CborIpldStore
}

// NewChainManager creates a new filecoin chain manager
func NewChainManager(cs *hamt.CborIpldStore) *ChainManager {
	return &ChainManager{
		KnownGoodBlocks: cid.NewSet(),
		cstore:          cs,
	}
}

// SetBestBlock sets the genesis block
func (s *ChainManager) SetBestBlock(ctx context.Context, b *types.Block) error {
	_, err := s.cstore.Put(ctx, b)
	if err != nil {
		return errors.Wrap(err, "failed to put block to disk")
	}
	s.BestBlock = b // TODO: make a copy?
	s.KnownGoodBlocks.Add(b.Cid())
	return nil
}

// ProcessNewBlock sends a new block to the chain manager. If the block is
// better than our current best, it is accepted as our new best block.
// Otherwise an error is returned explaining why it was not accepted
func (s *ChainManager) ProcessNewBlock(ctx context.Context, blk *types.Block) error {
	if err := s.validateBlock(ctx, blk); err != nil {
		return errors.Wrap(err, "validate block failed")
	}

	if blk.Score() > s.BestBlock.Score() {
		return s.acceptNewBestBlock(ctx, blk)
	}

	return fmt.Errorf("new block not better than current block (%d <= %d)",
		blk.Score(), s.BestBlock.Score())
}

// acceptNewBestBlock sets the given block as our current 'best chain' block
func (s *ChainManager) acceptNewBestBlock(ctx context.Context, blk *types.Block) error {
	if err := s.SetBestBlock(ctx, blk); err != nil {
		return err
	}

	// TODO: when we have transactions, adjust the local transaction mempool to
	// remove any transactions contained in our chosen chain, and if we dropped
	// any blocks (reorg, longer chain choice) re-add any missing txs back into
	// the mempool

	log.Infof("accepted new block, [s=%d, h=%s]", blk.Score(), blk.Cid())
	return nil
}

func (s *ChainManager) fetchBlock(ctx context.Context, c *cid.Cid) (*types.Block, error) {
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
// properly filled out and its signature is correct. Checking the validity of
// state changes must be done separately and only once the state of the
// previous block has been validated.
func (s *ChainManager) validateBlockStructure(ctx context.Context, b *types.Block) error {
	return nil
}

// TODO: this method really needs to be thought through carefully. Probably one
// of the most complicated bits of the system
func (s *ChainManager) validateBlock(ctx context.Context, b *types.Block) error {
	if err := s.validateBlockStructure(ctx, b); err != nil {
		return errors.Wrap(err, "check block valid failed")
	}

	if b.Score() <= s.BestBlock.Score() {
		// TODO: likely should still validate this chain and keep it around.
		// Someone else could mine on top of it
		return fmt.Errorf("new block is not better than our current block")
	}

	// TODO: should be some sort of limit here
	// Some implementations limit the length of a chain that can be swapped.
	// Historically, bitcoin does not, this is purely for religious and
	// idealogical reasons. In reality, if a weeks worth of blocks is about to
	// be reverted, the system should opt to halt, not just happily switch over
	// to an entirely different chain.
	var validating []*types.Block
	baseBlk := b
	for !s.KnownGoodBlocks.Has(baseBlk.Cid()) {
		validating = append(validating, baseBlk)

		next, err := s.fetchBlock(ctx, baseBlk.Parent)
		if err != nil {
			return errors.Wrap(err, "fetch block failed")
		}

		if err := s.validateBlockStructure(ctx, next); err != nil {
			return err
		}

		baseBlk = next
	}

	for i := len(validating) - 1; i >= 0; i-- {
		// TODO: check that state transitions are valid once we have them
		s.KnownGoodBlocks.Add(validating[i].Cid())
	}

	return nil
}
