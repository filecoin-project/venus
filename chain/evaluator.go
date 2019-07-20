package chain

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/metrics"
	"github.com/filecoin-project/go-filecoin/sampling"
	"github.com/filecoin-project/go-filecoin/types"
)

var syncOneTimer *metrics.Float64Timer

func init() {
	syncOneTimer = metrics.NewTimerMs("syncer/sync_one", "Duration of single tipset validation in milliseconds")
}

// Evaluator defines an interface for evaluating a chain
type Evaluator interface {
	Evaluate(context.Context, types.TipSet, []types.TipSet) error
	IsBadTipSet(types.TipSetKey) bool
	ExceedsFinalityLimit(ts types.TipSet) bool
}

type evaluatorChainReaderWriter interface {
	BlockHeight() (uint64, error)
	GetHead() types.TipSetKey
	GetTipSet(tsKey types.TipSetKey) (types.TipSet, error)
	GetTipSetAndStatesByParentsAndHeight(pTsKey string, h uint64) ([]*TipSetAndState, error)
	GetTipSetStateRoot(tsKey types.TipSetKey) (cid.Cid, error)
	HasTipSetAndStatesWithParentsAndHeight(pTsKey string, h uint64) bool
	PutTipSetAndState(ctx context.Context, tsas *TipSetAndState) error
	SetHead(ctx context.Context, s types.TipSet) error
}

// StateEvaluator is a fancy wrapper around calls to expected consensus
type StateEvaluator struct {
	// Provides and stores validated tipsets and their state roots.
	chainStore evaluatorChainReaderWriter
	// Evaluates tipset messages and stores the resulting states.
	chainTransitioner syncStateTransitioner
	// badTipSetCache is used to filter out collections of invalid blocks.
	badTipSets *badTipSetCache
}

type syncStateTransitioner interface {
	// RunStateTransition returns the state root CID resulting from applying the input ts to the
	// prior `stateRoot`.  It returns an error if the transition is invalid.
	RunStateTransition(ctx context.Context, ts types.TipSet, ancestors []types.TipSet, stateID cid.Cid) (cid.Cid, error)

	// IsHeaver returns 1 if tipset a is heavier than tipset b and -1 if
	// tipset b is heavier than tipset a.
	IsHeavier(ctx context.Context, a, b types.TipSet, aStateID, bStateID cid.Cid) (bool, error)
}

// NewChainStateEvaluator returns a new StateEvaluator.
func NewChainStateEvaluator(e syncStateTransitioner, store evaluatorChainReaderWriter) *StateEvaluator {
	return &StateEvaluator{
		chainStore:        store,
		chainTransitioner: e,
		badTipSets: &badTipSetCache{
			bad: make(map[string]struct{}),
		},
	}
}

// IsBadTipSet returns true if the tipset `ts` is known to be bad, true otherwise.
func (cse *StateEvaluator) IsBadTipSet(ts types.TipSetKey) bool {
	return cse.badTipSets.Has(ts.String())
}

// Evaluate is a wrapper around calls to consensus, it will either sync the chain `chain` to the store
// or return an error
func (cse *StateEvaluator) Evaluate(ctx context.Context, parent types.TipSet, chain []types.TipSet) error {
	// TODO add this as a sanity check
	/*
		if p, err := chain[0].Parents(); err != nil && p.ContainsAll(parent.Key()) {
			panic("boom"
		}
	*/
	head := chain[0].Key()
	// Try adding the tipsets of the chain to the store, checking for new
	// heaviest tipsets.
	for i, ts := range chain {
		// TODO: this "i==0" leaks EC specifics into syncer abstraction
		// for the sake of efficiency, consider plugging up this leak.
		if i == 0 {
			wts, err := cse.widen(ctx, ts)
			if err != nil {
				return err
			}
			if wts.Defined() {
				logSyncer.Debug("attempt to sync after widen")
				err = cse.syncOne(ctx, parent, wts)
				if err != nil {
					return err
				}
			}
		}
		if err := cse.syncOne(ctx, parent, ts); err != nil {
			// While `syncOne` can indeed fail for reasons other than consensus,
			// adding to the badTipSets at this point is the simplest, since we
			// have access to the chain. If syncOne fails for non-consensus reasons,
			// there is no assumption that the running node's data is valid at all,
			// so we don't really lose anything with this simplification.
			cse.badTipSets.AddChain(chain[i:])
			return err
		}
		if i%500 == 0 {
			logSyncer.Infof("processing block %d of %v for chain with head at %v", i, len(chain), head.String())
		}
		parent = ts
	}
	return nil
}

func (cse *StateEvaluator) syncOne(ctx context.Context, parent, next types.TipSet) error {
	priorHeadKey := cse.chainStore.GetHead()

	// if tipset is already priorHeadKey, we've been here before. do nothing.
	if priorHeadKey.Equals(next.Key()) {
		return nil
	}

	stopwatch := syncOneTimer.Start(ctx)
	defer stopwatch.Stop(ctx)

	// Lookup parent state root. It is guaranteed by the syncer that it is in the chainStore.
	stateRoot, err := cse.chainStore.GetTipSetStateRoot(parent.Key())
	if err != nil {
		return err
	}

	// Gather ancestor chain needed to process state transition.
	h, err := next.Height()
	if err != nil {
		return err
	}
	newBlockHeight := types.NewBlockHeight(h)
	ancestorHeight := types.NewBlockHeight(consensus.AncestorRoundsNeeded)
	ancestors, err := GetRecentAncestors(ctx, parent, cse.chainStore, newBlockHeight, ancestorHeight, sampling.LookbackParameter)
	if err != nil {
		return err
	}

	// Run a state transition to validate the tipset and compute
	// a new state to add to the store.
	root, err := cse.chainTransitioner.RunStateTransition(ctx, next, ancestors, stateRoot)
	if err != nil {
		return err
	}
	err = cse.chainStore.PutTipSetAndState(ctx, &TipSetAndState{
		TipSet:          next,
		TipSetStateRoot: root,
	})
	if err != nil {
		return err
	}
	logSyncer.Debugf("Successfully updated store with %s", next.String())

	// TipSet is validated and added to store, now check if it is the heaviest.
	nextParentStateID, err := cse.chainStore.GetTipSetStateRoot(parent.Key())
	if err != nil {
		return err
	}

	headTipSet, err := cse.chainStore.GetTipSet(priorHeadKey)
	if err != nil {
		return err
	}
	headParentKey, err := headTipSet.Parents()
	if err != nil {
		return err
	}

	var headParentStateID cid.Cid
	if !headParentKey.Empty() { // head is not genesis
		headParentStateID, err = cse.chainStore.GetTipSetStateRoot(headParentKey)
		if err != nil {
			return err
		}
	}

	heavier, err := cse.chainTransitioner.IsHeavier(ctx, next, headTipSet, nextParentStateID, headParentStateID)
	if err != nil {
		return err
	}

	// If it is the heaviest update the chainStore.
	if heavier {
		if err = cse.chainStore.SetHead(ctx, next); err != nil {
			return err
		}
		// Gather the entire new chain for reorg comparison and logging.
		cse.logReorg(ctx, headTipSet, next)
	}

	return nil
}

// widen computes a tipset implied by the input tipset and the store that
// could potentially be the heaviest tipset. In the context of EC, widen
// returns the union of the input tipset and the biggest tipset with the same
// parents from the store.
// TODO: this leaks EC abstractions into the syncer, we should think about this.
func (cse *StateEvaluator) widen(ctx context.Context, ts types.TipSet) (types.TipSet, error) {
	// Lookup tipsets with the same parents from the store.
	parentSet, err := ts.Parents()
	if err != nil {
		return types.UndefTipSet, err
	}
	height, err := ts.Height()
	if err != nil {
		return types.UndefTipSet, err
	}
	if !cse.chainStore.HasTipSetAndStatesWithParentsAndHeight(parentSet.String(), height) {
		return types.UndefTipSet, nil
	}
	candidates, err := cse.chainStore.GetTipSetAndStatesByParentsAndHeight(parentSet.String(), height)
	if err != nil {
		return types.UndefTipSet, err
	}
	if len(candidates) == 0 {
		return types.UndefTipSet, nil
	}

	// Only take the tipset with the most blocks (this is EC specific logic)
	max := candidates[0].TipSet
	for _, candidate := range candidates[0:] {
		if candidate.TipSet.Len() > max.Len() {
			max = candidate.TipSet
		}
	}

	// Form a new tipset from the union of ts and the largest in the store, de-duped.
	var blockSlice []*types.Block
	blockCids := make(map[cid.Cid]struct{})
	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)
		blockCids[blk.Cid()] = struct{}{}
		blockSlice = append(blockSlice, blk)
	}
	for i := 0; i < max.Len(); i++ {
		blk := max.At(i)
		if _, found := blockCids[blk.Cid()]; !found {
			blockSlice = append(blockSlice, blk)
			blockCids[blk.Cid()] = struct{}{}
		}
	}
	wts, err := types.NewTipSet(blockSlice...)
	if err != nil {
		return types.UndefTipSet, err
	}

	// check that the tipset is distinct from the input and tipsets from the store.
	if wts.String() == ts.String() || wts.String() == max.String() {
		return types.UndefTipSet, nil
	}

	return wts, nil
}

func (cse *StateEvaluator) logReorg(ctx context.Context, curHead, newHead types.TipSet) {
	curHeadIter := IterAncestors(ctx, cse.chainStore, curHead)
	newHeadIter := IterAncestors(ctx, cse.chainStore, newHead)
	commonAncestor, err := FindCommonAncestor(curHeadIter, newHeadIter)
	if err != nil {
		// Should never get here because reorgs should always have a
		// common ancestor..
		logSyncer.Warningf("unexpected error when running FindCommonAncestor for reorg log: %s", err.Error())
		return
	}

	reorg := IsReorg(curHead, newHead, commonAncestor)
	if reorg {
		reorgCnt.Inc(ctx, 1)
		dropped, added, err := ReorgDiff(curHead, newHead, commonAncestor)
		if err == nil {
			logSyncer.Infof("reorg dropping %d height and adding %d height from %s to %s", dropped, added, curHead.String(), newHead.String())
		} else {
			logSyncer.Infof("reorg from %s to %s", curHead.String(), newHead.String())
			logSyncer.Errorf("unexpected error from ReorgDiff during log: %s", err.Error())
		}
	}
}

// ExceedsFinalityLimit return true if `ts` is greater than FinalityLimit from the
// stores current head.
func (cse *StateEvaluator) ExceedsFinalityLimit(ts types.TipSet) bool {
	tsh, err := ts.Height()
	if err != nil {
		return false
	}
	blockHeight, _ := cse.chainStore.BlockHeight()
	finalityHeight := types.NewBlockHeight(blockHeight).Add(types.NewBlockHeight(uint64(FinalityLimit)))
	logSyncer.Debugf("Tipset: %s Height: %d StoreHeight: %d, FinalityHeight: %s", ts, tsh, blockHeight, finalityHeight.String())
	return types.NewBlockHeight(tsh).GreaterThan(finalityHeight)
}
