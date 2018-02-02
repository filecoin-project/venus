package state

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

var log = logging.Logger("state")

// StateManager manages the current state of the chain and handles validating
// and applying updates.
// Should be safe for concurrent access (This may not yet be the case)
type StateManager struct {
	BestBlock *types.Block

	// TODO: need some sync stuff here. Some of these fields get access in a
	// racy way right now. These fields need to be safe for concurrent access.
	// TODO: this should probably be an LRU
	KnownGoodBlocks *cid.Set

	cs *hamt.CborIpldStore
}

// NewStateManager creates a new filecoin state manager
func NewStateManager(cs *hamt.CborIpldStore) *StateManager {
	return &StateManager{
		KnownGoodBlocks: cid.NewSet(),
		cs:              cs,
	}
}

// SetGenesisBlock sets the genesis block
func (s *StateManager) SetBestBlock(ctx context.Context, b *types.Block) error {
	s.BestBlock = b // TODO: make a copy?
	s.KnownGoodBlocks.Add(b.Cid())
	_, err := s.cs.Put(ctx, b)
	if err != nil {
		return errors.Wrap(err, "failed to put block to disk")
	}
	return nil
}

// ProcessNewBlock sends a new block to the state manager. If the block is
// better than our current best, it is accepted as our new best block.
// Otherwise an error is returned explaining why it was not accepted
func (s *StateManager) ProcessNewBlock(ctx context.Context, blk *types.Block) error {
	if err := s.validateBlock(ctx, blk); err != nil {
		return errors.Wrap(err, "validate block failed")
	}

	if blk.Score() > s.BestBlock.Score() {
		return s.acceptNewBlock(ctx, blk)
	}

	return fmt.Errorf("new block not better than current block (%d <= %d)",
		blk.Score(), s.BestBlock.Score())
}

// acceptNewBlock sets the given block as our current 'best chain' block
func (s *StateManager) acceptNewBlock(ctx context.Context, blk *types.Block) error {
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

func (s *StateManager) fetchBlock(ctx context.Context, c *cid.Cid) (*types.Block, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	var blk types.Block
	if err := s.cs.Get(ctx, c, &blk); err != nil {
		return nil, err
	}

	return &blk, nil
}

// checkBlockValid verifies that this block, on its own, is structurally and
// cryptographically valid. This means checking that all of its fields are
// properly filled out and its signature is correct. Checking the validity of
// state changes must be done separately and only once the state of the
// previous block has been validated.
func (s *StateManager) checkBlockValid(ctx context.Context, b *types.Block) error {
	return nil
}

// TODO: this method really needs to be thought through carefully. Probably one
// of the most complicated bits of the system
func (s *StateManager) validateBlock(ctx context.Context, b *types.Block) error {
	if err := s.checkBlockValid(ctx, b); err != nil {
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

		if err := s.checkBlockValid(ctx, next); err != nil {
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
