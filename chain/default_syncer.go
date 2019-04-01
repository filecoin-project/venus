package chain

import (
	"context"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/sampling"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

// The amount of time the syncer will wait while fetching the blocks of a
// tipset over the network.
var blkWaitTime = 30 * time.Second
var (
	// ErrChainHasBadTipSet is returned when the syncer traverses a chain with a cached bad tipset.
	ErrChainHasBadTipSet = errors.New("input chain contains a cached bad tipset")
	// ErrNewChainTooLong is returned when processing a fork that split off from the main chain too many blocks ago.
	ErrNewChainTooLong = errors.New("input chain forked from best chain too far in the past")
	// ErrUnexpectedStoreState indicates that the syncer's chain store is violating expected invariants.
	ErrUnexpectedStoreState = errors.New("the chain store is in an unexpected state")
)

var logSyncer = logging.Logger("chain.syncer")

type syncFetcher interface {
	GetBlocks(context.Context, []cid.Cid) ([]*types.Block, error)
}

// DefaultSyncer updates its chain.Store according to the methods of its
// consensus.Protocol.  It uses a bad tipset cache and a limit on new
// blocks to traverse during chain collection.  The DefaultSyncer can query the
// network for blocks.  The DefaultSyncer maintains the following invariant on
// its store: all tipsets that pass the syncer's validity checks are added to the
// chain store, and their state is added to stateStore.
//
// Ideally the code that syncs the chain according to consensus rules should
// be independent of any particular implementation of consensus.  Currently the
// DefaultSyncer is coupled to details of Expected Consensus. This dependence
// exists in the widen function, the fact that widen is called on only one
// tipset in the incoming chain, and assumptions regarding the existence of
// grandparent state in the store.
type DefaultSyncer struct {
	// This mutex ensures at most one call to HandleNewTipset executes at
	// any time.  This is important because at least two sections of the
	// code otherwise have races:
	// 1. syncOne assumes that chainStore.Head() does not change when
	// comparing tipset weights and updating the store
	// 2. HandleNewTipset assumes that calls to widen and then syncOne
	// are not run concurrently with other calls to widen to ensure
	// that the syncer always finds the heaviest existing tipset.
	mu sync.Mutex
	// fetcher is the networked block fetching service for fetching blocks
	// and messages.
	fetcher syncFetcher
	// stateStore is the cborStore used for reading and writing state root
	// to ipld object mappings.
	stateStore *hamt.CborIpldStore
	// badTipSetCache is used to filter out collections of invalid blocks.
	badTipSets *badTipSetCache
	consensus  consensus.Protocol
	chainStore Store
}

var _ Syncer = (*DefaultSyncer)(nil)

// NewDefaultSyncer constructs a DefaultSyncer ready for use.
func NewDefaultSyncer(cst *hamt.CborIpldStore, c consensus.Protocol, s Store, f syncFetcher) *DefaultSyncer {
	return &DefaultSyncer{
		fetcher:    f,
		stateStore: cst,
		badTipSets: &badTipSetCache{
			bad: make(map[string]struct{}),
		},
		consensus:  c,
		chainStore: s,
	}
}

// getBlksMaybeFromNet resolves cids of blocks.  It gets blocks through the
// fetcher.  The fetcher wraps a bitswap session which wraps a bitswap exchange,
// and the bitswap exchange wraps the node's shared blockstore.  So if blocks
// are available in the node's blockstore they will be resolved locally, and
// otherwise resolved over the network.  This method will timeout if blocks
// are unavailable.  This method is all or nothing, it will error if any of the
// blocks cannot be resolved.
func (syncer *DefaultSyncer) getBlksMaybeFromNet(ctx context.Context, blkCids []cid.Cid) ([]*types.Block, error) {
	ctx, cancel := context.WithTimeout(ctx, blkWaitTime)
	defer cancel()

	return syncer.fetcher.GetBlocks(ctx, blkCids)
}

// collectChain resolves the cids of the head tipset and its ancestors to
// blocks until it resolves a tipset with a parent contained in the Store. It
// returns the chain of new incompletely validated tipsets and the id of the
// parent tipset already synced into the store.  collectChain resolves cids
// from the syncer's fetcher.  In production the fetcher wraps a bitswap
// session.  collectChain errors if any set of cids in the chain resolves to
// blocks that do not form a tipset, or if any tipset has already been recorded
// as the head of an invalid chain.  collectChain is the entrypoint to the code
// that interacts with the network. It does NOT add tipsets to the chainStore..
func (syncer *DefaultSyncer) collectChain(ctx context.Context, tipsetCids types.SortedCidSet) ([]types.TipSet, error) {
	var chain []types.TipSet
	defer logSyncer.Info("chain synced")
	for {
		var blks []*types.Block
		// check the cache for bad tipsets before doing anything
		tsKey := tipsetCids.String()

		// Finish traversal if the tipset made is tracked in the store.
		if syncer.chainStore.HasTipSetAndState(ctx, tsKey) {
			return chain, nil
		}

		logSyncer.Debugf("CollectChain next link: %s", tsKey)

		if syncer.badTipSets.Has(tsKey) {
			return nil, ErrChainHasBadTipSet
		}

		blks, err := syncer.getBlksMaybeFromNet(ctx, tipsetCids.ToSlice())
		if err != nil {
			return nil, err
		}

		ts, err := syncer.consensus.NewValidTipSet(ctx, blks)
		if err != nil {
			syncer.badTipSets.Add(tsKey)
			syncer.badTipSets.AddChain(chain)
			return nil, err
		}

		height, _ := ts.Height()
		if len(chain)%500 == 0 {
			logSyncer.Infof("syncing the chain, currently at block height %d", height)
		}

		// Update values to traverse next tipset
		chain = append([]types.TipSet{ts}, chain...)
		tipsetCids, err = ts.Parents()
		if err != nil {
			return nil, err
		}
	}
}

// tipSetState returns the state resulting from applying the input tipset to
// the chain.  Precondition: the tipset must be in the store
func (syncer *DefaultSyncer) tipSetState(ctx context.Context, tsKey types.SortedCidSet) (state.Tree, error) {
	if !syncer.chainStore.HasTipSetAndState(ctx, tsKey.String()) {
		return nil, errors.Wrap(ErrUnexpectedStoreState, "parent tipset must be in the store")
	}
	tsas, err := syncer.chainStore.GetTipSetAndState(ctx, tsKey)
	if err != nil {
		return nil, err
	}
	st, err := state.LoadStateTree(ctx, syncer.stateStore, tsas.TipSetStateRoot, builtin.Actors)
	if err != nil {
		return nil, err
	}
	return st, nil
}

// syncOne syncs a single tipset with the chain store. syncOne calculates the
// parent state of the tipset and calls into consensus to run a state transition
// in order to validate the tipset.  In the case the input tipset is valid,
// syncOne calls into consensus to check its weight, and then updates the head
// of the store if this tipset is the heaviest.
//
// Precondition: the caller of syncOne must hold the syncer's lock (syncer.mu) to
// ensure head is not modified by another goroutine during run.
func (syncer *DefaultSyncer) syncOne(ctx context.Context, parent, next types.TipSet) error {
	head := syncer.chainStore.Head()

	// if tipset is already head, we've been here before. do nothing.
	if head.Equals(next) {
		return nil
	}

	// Lookup parent state. It is guaranteed by the syncer that it is in
	// the chainStore.
	st, err := syncer.tipSetState(ctx, parent.ToSortedCidSet())
	if err != nil {
		return err
	}

	// Gather ancestor chain needed to process state transition.
	h, err := next.Height()
	if err != nil {
		return err
	}
	newBlockHeight := types.NewBlockHeight(h)
	ancestors, err := GetRecentAncestors(ctx, parent, syncer.chainStore, newBlockHeight, consensus.AncestorRoundsNeeded, sampling.LookbackParameter)
	if err != nil {
		return err
	}

	// Run a state transition to validate the tipset and compute
	// a new state to add to the store.
	st, err = syncer.consensus.RunStateTransition(ctx, next, ancestors, st)
	if err != nil {
		return err
	}
	root, err := st.Flush(ctx)
	if err != nil {
		return err
	}
	err = syncer.chainStore.PutTipSetAndState(ctx, &TipSetAndState{
		TipSet:          next,
		TipSetStateRoot: root,
	})
	if err != nil {
		return err
	}
	logSyncer.Debugf("Successfully updated store with %s", next.String())

	// TipSet is validated and added to store, now check if it is the heaviest.
	// If it is the heaviest update the chainStore.
	nextParentSt, err := syncer.tipSetState(ctx, parent.ToSortedCidSet()) // call again to get a copy
	if err != nil {
		return err
	}
	headTipSetAndState, err := syncer.chainStore.GetTipSetAndState(ctx, head)
	if err != nil {
		return err
	}
	headParentCids, err := headTipSetAndState.TipSet.Parents()
	if err != nil {
		return err
	}
	var headParentSt state.Tree
	if headParentCids.Len() != 0 { // head is not genesis
		headParentSt, err = syncer.tipSetState(ctx, headParentCids)
		if err != nil {
			return err
		}
	}

	heavier, err := syncer.consensus.IsHeavier(ctx, next, headTipSetAndState.TipSet, nextParentSt, headParentSt)
	if err != nil {
		return err
	}

	if heavier {
		// Gather the entire new chain for reorg comparison.
		// See Issue #2151 for making this scalable.
		newChain, err := CollectTipSetsOfHeightAtLeast(ctx, syncer.chainStore.BlockHistory(ctx, &parent), types.NewBlockHeight(uint64(0)))
		if err != nil {
			return err
		}
		newChain = append(newChain, next)
		if IsReorg(headTipSetAndState.TipSet, newChain) {
			logSyncer.Infof("reorg occurring while switching from %s to %s", headTipSetAndState.TipSet.String(), next.String())
		}
		if err = syncer.chainStore.SetHead(ctx, next); err != nil {
			return err
		}
	}

	return nil
}

// widen computes a tipset implied by the input tipset and the store that
// could potentially be the heaviest tipset. In the context of EC, widen
// returns the union of the input tipset and the biggest tipset with the same
// parents from the store.
// TODO: this leaks EC abstractions into the syncer, we should think about this.
func (syncer *DefaultSyncer) widen(ctx context.Context, ts types.TipSet) (types.TipSet, error) {
	// Lookup tipsets with the same parents from the store.
	parentSet, err := ts.Parents()
	if err != nil {
		return nil, err
	}
	height, err := ts.Height()
	if err != nil {
		return nil, err
	}
	if !syncer.chainStore.HasTipSetAndStatesWithParentsAndHeight(ctx, parentSet.String(), height) {
		return nil, nil
	}
	candidates, err := syncer.chainStore.GetTipSetAndStatesByParentsAndHeight(ctx, parentSet.String(), height)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	// Only take the tipset with the most blocks (this is EC specific logic)
	max := candidates[0]
	for _, candidate := range candidates[0:] {
		if len(candidate.TipSet) > len(max.TipSet) {
			max = candidate
		}
	}

	// Add blocks of the biggest tipset in the store to a copy of ts
	wts := ts.Clone()
	for _, blk := range max.TipSet {
		if err = wts.AddBlock(blk); err != nil {
			return nil, err
		}
	}

	// check that the tipset is distinct from the input and tipsets from the store.
	if wts.String() == ts.String() || wts.String() == max.TipSet.String() {
		return nil, nil
	}

	return wts, nil
}

// HandleNewTipset extends the Syncer's chain store with the given tipset if they
// represent a valid extension. It limits the length of new chains it will
// attempt to validate and caches invalid blocks it has encountered to
// help prevent DOS.
func (syncer *DefaultSyncer) HandleNewTipset(ctx context.Context, tipsetCids types.SortedCidSet) error {
	logSyncer.Debugf("trying to sync %v\n", tipsetCids)

	// This lock could last a long time as we fetch all the blocks needed to block the chain.
	// This is justified because the app is pretty useless until it is synced.
	// It's better for multiple calls to wait here than to try to fetch the chain independently.
	syncer.mu.Lock()
	defer syncer.mu.Unlock()

	// If the store already has all these blocks the syncer is finished.
	if syncer.chainStore.HasAllBlocks(ctx, tipsetCids.ToSlice()) {
		return nil
	}

	// Walk the chain given by the input blocks back to a known tipset in
	// the store. This is the only code that may go to the network to
	// resolve cids to blocks.
	chain, err := syncer.collectChain(ctx, tipsetCids)
	if err != nil {
		return err
	}
	parentCids, err := chain[0].Parents()
	if err != nil {
		return err
	}
	parentTsas, err := syncer.chainStore.GetTipSetAndState(ctx, parentCids)
	if err != nil {
		return err
	}
	parent := parentTsas.TipSet

	// Try adding the tipsets of the chain to the store, checking for new
	// heaviest tipsets.
	for i, ts := range chain {
		// TODO: this "i==0" leaks EC specifics into syncer abstraction
		// for the sake of efficiency, consider plugging up this leak.
		if i == 0 {
			wts, err := syncer.widen(ctx, ts)
			if err != nil {
				return err
			}
			if wts != nil {
				logSyncer.Debug("attempt to sync after widen")
				err = syncer.syncOne(ctx, parent, wts)
				if err != nil {
					return err
				}
			}
		}
		if err = syncer.syncOne(ctx, parent, ts); err != nil {
			// While `syncOne` can indeed fail for reasons other than consensus,
			// adding to the badTipSets at this point is the simplest, since we
			// have access to the chain. If syncOne fails for non-consensus reasons,
			// there is no assumption that the running node's data is valid at all,
			// so we don't really lose anything with this simplification.
			syncer.badTipSets.AddChain(chain[i:])
			return err
		}
		parent = ts
	}
	return nil
}
