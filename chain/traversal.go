package chain

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

// TipSetProvider provides tipsets for traversal.
type TipSetProvider interface {
	GetTipSet(tsKey types.TipSetKey) (types.TipSet, error)
}

// IterAncestors returns an iterator over tipset ancestors, yielding first the start tipset and
// then its parent tipsets until (and including) the genesis tipset.
func IterAncestors(ctx context.Context, store TipSetProvider, start types.TipSet) *TipsetIterator {
	return &TipsetIterator{ctx, store, start}
}

// TipsetIterator is an iterator over tipsets.
type TipsetIterator struct {
	ctx   context.Context
	store TipSetProvider
	value types.TipSet
}

// Value returns the iterator's current value, if not Complete().
func (it *TipsetIterator) Value() types.TipSet {
	return it.value
}

// Complete tests whether the iterator is exhausted.
func (it *TipsetIterator) Complete() bool {
	return !it.value.Defined()
}

// Next advances the iterator to the next value.
func (it *TipsetIterator) Next() error {
	select {
	case <-it.ctx.Done():
		return it.ctx.Err()
	default:
		parentKey, err := it.value.Parents()
		// Parents is empty (without error) for the genesis tipset.
		if err != nil || parentKey.Len() == 0 {
			it.value = types.UndefTipSet
		} else {
			it.value, err = it.store.GetTipSet(parentKey)
		}
		return err
	}
}

// IterTickets returns an iterator over the ancestral ticketchain yielding
// first the tickets of the start tipset and then its parent tipset's tickets
// until and including the genesis ticket.
func IterTickets(ctx context.Context, store TipSetProvider, start types.TipSet) *TicketIterator {
	startIdx := len(start.At(0).Tickets) - 1
	return &TicketIterator{
		tips:  &TipsetIterator{ctx, store, start},
		idx:   startIdx,
		value: start.At(0).Tickets[startIdx],
	}
}

// TicketIterator is an iterator over tickets.
type TicketIterator struct {
	tips  *TipsetIterator
	idx   int
	value types.Ticket
}

// Value returns the iterator's current value, if not Complete().
func (it *TicketIterator) Value() types.Ticket {
	return it.value
}

// Complete tests whether the iterator is exhausted.
func (it *TicketIterator) Complete() bool {
	return it.tips.Complete()
}

// Next advances the iterator to the next ticket. Tickets are taken from the
// block with the minimum final ticket in its ticket array
// https://github.com/filecoin-project/specs/blob/master/definitions.md#ticket-chain
func (it *TicketIterator) Next() error {
	if it.tips.Complete() {
		return errors.NewError("calling next on complete ticket iterator")
	}

	// Advance internal tipset iterator
	if it.idx == 0 {
		if err := it.tips.Next(); err != nil {
			return err
		}
		if it.tips.Complete() {
			it.value = types.UndefTicket
			return nil
		}
		it.idx = len(it.tips.Value().At(0).Tickets) - 1
		it.value = it.tips.Value().At(0).Tickets[it.idx]
		}
		return nil 
	}

	// Advance internally along min-ticket block's ticket array
	it.idx = it.idx - 1
	it.value = it.tips.Value().At(0).Tickets[it.idx]
	return nil
}

// BlockProvider provides blocks.
type BlockProvider interface {
	GetBlock(ctx context.Context, cid cid.Cid) (*types.Block, error)
}

// LoadTipSetBlocks loads all the blocks for a tipset from the store.
func LoadTipSetBlocks(ctx context.Context, store BlockProvider, key types.TipSetKey) (types.TipSet, error) {
	var blocks []*types.Block
	for it := key.Iter(); !it.Complete(); it.Next() {
		blk, err := store.GetBlock(ctx, it.Value())
		if err != nil {
			return types.UndefTipSet, err
		}
		blocks = append(blocks, blk)
	}
	return types.NewTipSet(blocks...)
}

type tipsetFromBlockProvider struct {
	ctx    context.Context // Context to use when loading blocks
	blocks BlockProvider   // Provides blocks
}

// TipSetProviderFromBlocks builds a tipset provider backed by a block provider.
// Blocks will be loaded with the provided context, since GetTipSet does not accept a
// context parameter. This can and should be removed when GetTipSet does take a context.
func TipSetProviderFromBlocks(ctx context.Context, blocks BlockProvider) TipSetProvider {
	return &tipsetFromBlockProvider{ctx, blocks}
}

// GetTipSet loads the blocks for a tipset.
func (p *tipsetFromBlockProvider) GetTipSet(tsKey types.TipSetKey) (types.TipSet, error) {
	return LoadTipSetBlocks(p.ctx, p.blocks, tsKey)
}

// CollectTipsToCommonAncestor traverses chains from two tipsets (called old and new) until their common
// ancestor, collecting all tipsets that are in one chain but not the other.
// The resulting lists of tipsets are ordered by decreasing height.
func CollectTipsToCommonAncestor(ctx context.Context, store TipSetProvider, oldHead, newHead types.TipSet) (oldTips, newTips []types.TipSet, err error) {
	oldIter := IterAncestors(ctx, store, oldHead)
	newIter := IterAncestors(ctx, store, newHead)

	commonAncestor, err := FindCommonAncestor(oldIter, newIter)
	if err != nil {
		return
	}
	commonHeight, err := commonAncestor.Height()
	if err != nil {
		return
	}

	// Refresh iterators modified by FindCommonAncestors
	oldIter = IterAncestors(ctx, store, oldHead)
	newIter = IterAncestors(ctx, store, newHead)

	// Add 1 to the height argument so that the common ancestor is not
	// included in the outputs.
	oldTips, err = CollectTipSetsOfHeightAtLeast(ctx, oldIter, types.NewBlockHeight(commonHeight+uint64(1)))
	if err != nil {
		return
	}
	newTips, err = CollectTipSetsOfHeightAtLeast(ctx, newIter, types.NewBlockHeight(commonHeight+uint64(1)))
	return
}
