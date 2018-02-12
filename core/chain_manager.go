package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"
	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	hamt "gx/ipfs/QmdBXcN47jVwKLwSyN9e9xYVZ7WcAWgQ5N4cmNw7nzWq2q/go-hamt-ipld"

	state "github.com/filecoin-project/go-filecoin/state"
	types "github.com/filecoin-project/go-filecoin/types"
)

var log = logging.Logger("chain")

var (
	ErrStateRootMismatch = errors.New("blocks state root does not match computed result")
	ErrInvalidBase       = errors.New("block does not connect to a known good chain")
)

// BlockProcessResult signifies the outcome of processing a given block.
type BlockProcessResult int

const (
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

	// KnownGoodBlocks is the set of 'good blocks'. It is a cache to prevent us
	// from having to rescan parts of the blockchain when determining the
	// validity of a given chain.
	// In the future we will need a more sophisticated mechanism here.
	// TODO: this should probably be an LRU, needs more consideration.
	// For example, the genesis block should always be considered a "good" block.
	KnownGoodBlocks SyncCidSet

	cstore *hamt.CborIpldStore
}

// NewChainManager creates a new filecoin chain manager
func NewChainManager(cs *hamt.CborIpldStore) *ChainManager {
	cm := &ChainManager{
		cstore:    cs,
		processor: ProcessBlock,
	}
	cm.KnownGoodBlocks.set = cid.NewSet()

	return cm
}

func (s *ChainManager) Genesis(ctx context.Context, gen GenesisInitFunc) error {
	genesis, err := gen(s.cstore)
	if err != nil {
		return err
	}

	return s.setBestBlock(ctx, genesis)
}

// setBestBlock sets the best block. Should be used to either to set the
// genesis block, or to manually set the current selected chain for testing.
func (s *ChainManager) setBestBlock(ctx context.Context, b *types.Block) error {
	_, err := s.cstore.Put(ctx, b)
	if err != nil {
		return errors.Wrap(err, "failed to put block to disk")
	}
	s.bestBlock.blk = b // TODO: make a copy?
	s.KnownGoodBlocks.Add(b.Cid())

	return nil
}

// GetBestBlock returns the head of our currently selected 'best' chain.
func (s *ChainManager) GetBestBlock() *types.Block {
	s.bestBlock.Lock()
	defer s.bestBlock.Unlock()
	return s.bestBlock.blk // TODO: return immutable copy?
}

func (s *ChainManager) maybeAcceptBlock(ctx context.Context, blk *types.Block) (BlockProcessResult, error) {
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
	switch err := s.validateBlock(ctx, blk); err {
	default:
		return Unknown, errors.Wrap(err, "validate block failed")
	case ErrInvalidBase:
		return InvalidBase, ErrInvalidBase
	case nil:
		return s.maybeAcceptBlock(ctx, blk)
	}
}

// acceptNewBestBlock sets the given block as our current 'best chain' block
func (s *ChainManager) acceptNewBestBlock(ctx context.Context, blk *types.Block) (BlockProcessResult, error) {
	if err := s.setBestBlock(ctx, blk); err != nil {
		return Unknown, err
	}

	// TODO: when we have transactions, adjust the local transaction mempool to
	// remove any transactions contained in our chosen chain, and if we dropped
	// any blocks (reorg, longer chain choice) re-add any missing txs back into
	// the mempool

	log.Infof("accepted new block, [s=%d, h=%s]", blk.Score(), blk.Cid())
	return ChainAccepted, nil
}

// fetchBlock gets the requested block, either from disk or from the network.
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

	st, err := state.LoadTree(ctx, s.cstore, baseBlk.StateRoot)
	if err != nil {
		return err
	}

	for i := len(chain) - 1; i >= 0; i-- {
		cur := chain[i]
		if err := s.processor(ctx, cur, st); err != nil {
			return err
		}

		// TODO: check that state transitions are valid once we have them
		s.KnownGoodBlocks.Add(cur.Cid())
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
	for !s.KnownGoodBlocks.Has(baseBlk.Cid()) {
		if baseBlk.Parent == nil {
			return nil, nil, ErrInvalidBase
		}

		validating = append(validating, baseBlk)

		next, err := s.fetchBlock(ctx, baseBlk.Parent)
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
