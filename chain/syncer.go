package chain

import (
	"context"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/metrics"
	"github.com/filecoin-project/go-filecoin/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/net"
	"github.com/filecoin-project/go-filecoin/sampling"
	"github.com/filecoin-project/go-filecoin/types"
)

var reorgCnt *metrics.Int64Counter

func init() {
	reorgCnt = metrics.NewInt64Counter("chain/reorg_count", "The number of reorgs that have occured.")
}

// The amount of time the syncer will wait while fetching the blocks of a
// tipset over the network.
var blkWaitTime = 30 * time.Second

// FinalityLimit is the maximum number of blocks ahead of the current consensus
// chain height to accept once in caught up mode
var FinalityLimit = 600
var (
	// ErrChainHasBadTipSet is returned when the syncer traverses a chain with a cached bad tipset.
	ErrChainHasBadTipSet = errors.New("input chain contains a cached bad tipset")
	// ErrNewChainTooLong is returned when processing a fork that split off from the main chain too many blocks ago.
	ErrNewChainTooLong = errors.New("input chain forked from best chain too far in the past")
	// ErrUnexpectedStoreState indicates that the syncer's chain store is violating expected invariants.
	ErrUnexpectedStoreState = errors.New("the chain store is in an unexpected state")
)

var syncOneTimer *metrics.Float64Timer

func init() {
	syncOneTimer = metrics.NewTimerMs("syncer/sync_one", "Duration of single tipset validation in milliseconds")
}

var logSyncer = logging.Logger("chain.syncer")

// SyncMode represents which behavior mode the chain syncer is currently in. By
// default, the node starts in "Syncing" mode, and once it syncs the most
// recently generated block, it switches to "Caught Up" mode.
type SyncMode int

const (
	// Syncing indicates that the node was started recently and the chain is still
	// significantly behind the current consensus head.
	//
	// See the spec for more detail:
	// https://github.com/filecoin-project/specs/blob/master/sync.md#syncing-mode
	Syncing SyncMode = iota
	// CaughtUp indicates that the node has caught up with consensus head and as a
	// result can restrict which new blocks are accepted to mitigate consensus
	// attacks.
	//
	// See the spec for more detail:
	// https://github.com/filecoin-project/specs/blob/master/sync.md#caught-up-mode
	CaughtUp

	// Bad state, currently not used
	Unknown
)

type syncerChainReaderWriter interface {
	BlockHeight() (uint64, error)
	GetHead() types.TipSetKey
	GenesisCid() cid.Cid
	GetTipSet(tsKey types.TipSetKey) (types.TipSet, error)
	GetTipSetStateRoot(tsKey types.TipSetKey) (cid.Cid, error)
	HasTipSetAndState(ctx context.Context, tsKey string) bool
	PutTipSetAndState(ctx context.Context, tsas *TipSetAndState) error
	SetHead(ctx context.Context, s types.TipSet) error
	HasTipSetAndStatesWithParentsAndHeight(pTsKey string, h uint64) bool
	GetTipSetAndStatesByParentsAndHeight(pTsKey string, h uint64) ([]*TipSetAndState, error)
}

type syncStateEvaluator interface {
	// RunStateTransition returns the state root CID resulting from applying the input ts to the
	// prior `stateRoot`.  It returns an error if the transition is invalid.
	RunStateTransition(ctx context.Context, ts types.TipSet, ancestors []types.TipSet, stateID cid.Cid) (cid.Cid, error)

	// IsHeaver returns 1 if tipset a is heavier than tipset b and -1 if
	// tipset b is heavier than tipset a.
	IsHeavier(ctx context.Context, a, b types.TipSet, aStateID, bStateID cid.Cid) (bool, error)
}

// Syncer updates its chain.Store according to the methods of its
// consensus.Protocol.  It uses a bad tipset cache and a limit on new
// blocks to traverse during chain collection.  The Syncer can query the
// network for blocks.  The Syncer maintains the following invariant on
// its store: all tipsets that pass the syncer's validity checks are added to the
// chain store along with their state root CID.
//
// Ideally the code that syncs the chain according to consensus rules should
// be independent of any particular implementation of consensus.  Currently the
// Syncer is coupled to details of Expected Consensus. This dependence
// exists in the widen function, the fact that widen is called on only one
// tipset in the incoming chain, and assumptions regarding the existence of
// grandparent state in the store.
type Syncer struct {
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
	fetcher net.Fetcher
	// badTipSetCache is used to filter out collections of invalid blocks.
	badTipSets *badTipSetCache

	// Evaluates tipset messages and stores the resulting states.
	stateEvaluator syncStateEvaluator
	// Provides and stores validated tipsets and their state roots.
	chainStore syncerChainReaderWriter

	// syncMode is an enumerable indicating whether the chain is currently caught
	// up or still syncing. Presently, syncMode is always Syncing pending
	// implementation in issue #1160.
	//
	// TODO: https://github.com/filecoin-project/go-filecoin/issues/1160
	syncMode SyncMode

	// Peer Manager to track whoe we get tipsets from
	pm PeerManager
}

// NewSyncer constructs a Syncer ready for use.
func NewSyncer(e syncStateEvaluator, s syncerChainReaderWriter, f net.Fetcher, pm PeerManager, syncMode SyncMode) *Syncer {
	return &Syncer{
		fetcher: f,
		badTipSets: &badTipSetCache{
			bad: make(map[string]struct{}),
		},
		stateEvaluator: e,
		chainStore:     s,
		syncMode:       syncMode,
		pm:             pm,
	}
}

// fetchTipSetWithTimeout resolves a TipSetKey to a TipSet. This method will
// request the TipSet from the network if it is not found locally in the
// blockstore. This method is all or nothing, it will error if any of the cids
// that make up the requested tipset cannot be resolved.
func (syncer *Syncer) fetchTipSetWithTimeout(ctx context.Context, tsKey types.TipSetKey) (types.TipSet, error) {
	ctx, cancel := context.WithTimeout(ctx, blkWaitTime)
	defer cancel()

	tss, err := syncer.fetcher.FetchTipSets(ctx, tsKey, 1)
	if err != nil {
		return types.UndefTipSet, err
	}

	// TODO remove this when we land: https://github.com/filecoin-project/go-filecoin/issues/1105
	if len(tss) > 1 {
		panic("programmer error, FetchTipSets ignored recur parameter, returned more than 1 tipset")
	}

	return tss[0], nil
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
func (syncer *Syncer) collectChain(ctx context.Context, tsKey types.TipSetKey) (ts []types.TipSet, err error) {
	ctx, span := trace.StartSpan(ctx, "Syncer.collectChain")
	span.AddAttributes(trace.StringAttribute("tipset", tsKey.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	var chain []types.TipSet
	var count uint64
	defer logSyncer.Infof("chain fetch from network complete %s", tsKey.String())

	// Continue collecting the chain if we're either not yet caught up or the
	// number of new input blocks is less than the FinalityLimit constant.
	// Otherwise, halt assuming the new blocks come from an invalid chain.
	for (syncer.syncMode == Syncing) || !syncer.exceedsFinalityLimit(chain) {

		// Finish traversal if the tipset made is tracked in the store.
		if syncer.chainStore.HasTipSetAndState(ctx, tsKey.String()) {
			return chain, nil
		}

		logSyncer.Debugf("CollectChain next link: %s", tsKey.String())

		if syncer.badTipSets.Has(tsKey.String()) {
			return nil, ErrChainHasBadTipSet
		}

		ts, err := syncer.fetchTipSetWithTimeout(ctx, tsKey)
		if err != nil {
			return nil, err
		}

		count++
		if count%500 == 0 {
			logSyncer.Infof("fetching the chain, %d blocks fetched", count)
		}

		// Update values to traverse next tipset
		chain = append([]types.TipSet{ts}, chain...)
		tsKey, err = ts.Parents()
		if err != nil {
			return nil, err
		}
	}

	return nil, ErrNewChainTooLong
}

// syncOne syncs a single tipset with the chain store. syncOne calculates the
// parent state of the tipset and calls into consensus to run a state transition
// in order to validate the tipset.  In the case the input tipset is valid,
// syncOne calls into consensus to check its weight, and then updates the head
// of the store if this tipset is the heaviest.
//
// Precondition: the caller of syncOne must hold the syncer's lock (syncer.mu) to
// ensure head is not modified by another goroutine during run.
func (syncer *Syncer) syncOne(ctx context.Context, parent, next types.TipSet) error {
	priorHeadKey := syncer.chainStore.GetHead()

	// if tipset is already priorHeadKey, we've been here before. do nothing.
	if priorHeadKey.Equals(next.Key()) {
		return nil
	}

	stopwatch := syncOneTimer.Start(ctx)
	defer stopwatch.Stop(ctx)

	// Lookup parent state root. It is guaranteed by the syncer that it is in the chainStore.
	stateRoot, err := syncer.chainStore.GetTipSetStateRoot(parent.Key())
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
	ancestors, err := GetRecentAncestors(ctx, parent, syncer.chainStore, newBlockHeight, ancestorHeight, sampling.LookbackParameter)
	if err != nil {
		return err
	}

	// Run a state transition to validate the tipset and compute
	// a new state to add to the store.
	root, err := syncer.stateEvaluator.RunStateTransition(ctx, next, ancestors, stateRoot)
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
	nextParentStateID, err := syncer.chainStore.GetTipSetStateRoot(parent.Key())
	if err != nil {
		return err
	}

	headTipSet, err := syncer.chainStore.GetTipSet(priorHeadKey)
	if err != nil {
		return err
	}
	headParentKey, err := headTipSet.Parents()
	if err != nil {
		return err
	}

	var headParentStateID cid.Cid
	if !headParentKey.Empty() { // head is not genesis
		headParentStateID, err = syncer.chainStore.GetTipSetStateRoot(headParentKey)
		if err != nil {
			return err
		}
	}

	heavier, err := syncer.stateEvaluator.IsHeavier(ctx, next, headTipSet, nextParentStateID, headParentStateID)
	if err != nil {
		return err
	}

	// If it is the heaviest update the chainStore.
	if heavier {
		if err = syncer.chainStore.SetHead(ctx, next); err != nil {
			panic(err)
			return err
		}
		// Gather the entire new chain for reorg comparison and logging.
		syncer.logReorg(ctx, headTipSet, next)
	}

	return nil
}

func (syncer *Syncer) logReorg(ctx context.Context, curHead, newHead types.TipSet) {
	curHeadIter := IterAncestors(ctx, syncer.chainStore, curHead)
	newHeadIter := IterAncestors(ctx, syncer.chainStore, newHead)
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

// widen computes a tipset implied by the input tipset and the store that
// could potentially be the heaviest tipset. In the context of EC, widen
// returns the union of the input tipset and the biggest tipset with the same
// parents from the store.
// TODO: this leaks EC abstractions into the syncer, we should think about this.
func (syncer *Syncer) widen(ctx context.Context, ts types.TipSet) (types.TipSet, error) {
	// Lookup tipsets with the same parents from the store.
	parentSet, err := ts.Parents()
	if err != nil {
		return types.UndefTipSet, err
	}
	height, err := ts.Height()
	if err != nil {
		return types.UndefTipSet, err
	}
	if !syncer.chainStore.HasTipSetAndStatesWithParentsAndHeight(parentSet.String(), height) {
		return types.UndefTipSet, nil
	}
	candidates, err := syncer.chainStore.GetTipSetAndStatesByParentsAndHeight(parentSet.String(), height)
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

// HandleNewTipset extends the Syncer's chain store with the given tipset if they
// represent a valid extension. It limits the length of new chains it will
// attempt to validate and caches invalid blocks it has encountered to
// help prevent DOS.
func (syncer *Syncer) HandleNewTipset(ctx context.Context, from peer.ID, tsKey types.TipSetKey) (err error) {
	logSyncer.Infof("HandleNewTipSet: %s from peer: %s", tsKey.String(), from.Pretty())

	// Grab the full tipset
	blks, err := syncer.fetcher.GetBlocks(ctx, tsKey.ToSlice())
	if err != nil {
		return err
	}
	ts, err := types.NewTipSet(blks...)
	if err != nil {
		return err
	}
	syncer.pm.AddPeer(from, ts)

	go func() {
		syncer.mu.Lock()
		defer syncer.mu.Unlock()

		switch syncer.syncMode {
		case Syncing:
			if !(len(syncer.pm.Peers()) >= BootstrapThreshold) {
				logSyncer.Infof("Unable Bootstrap Sync, not enough peers to bootstrap from, current count %d, threshold: %d", len(syncer.pm.Peers()), BootstrapThreshold)
				return
			}

			logSyncer.Info("Going to bootstrap")
			if err := syncer.SyncBootstrap(ctx); err != nil {
				logSyncer.Errorf("Bootstrap sync error: %s", err.Error())
			}
			return
		case CaughtUp:
			logSyncer.Info("Going to caught up")
			if err := syncer.SyncCaughtUp(ctx, tsKey); err != nil {
				logSyncer.Errorf("CaughtUp error: %s", err.Error())
			}
			return
		case Unknown:
			panic("invalid syncer state")
		}
	}()

	return nil
}

// BootstrapThreshold is the minimum number of unique peers the syncer must know
// of before attempting to sync bootstrap
var BootstrapThreshold = 0

// BootstrapFetchWindow is the number of tipsets that will be requested at once
// when performing bootstrap sync
var BootstrapFetchWindow = 1

func (syncer *Syncer) SyncBootstrap(ctx context.Context) error {
	logSyncer.Info("Starting Bootstrap Sync")

	syncHead, err := syncer.pm.SelectHead()
	if err != nil {
		logSyncer.Errorf("Aborting Bootstrap Sync, failed to select head. Error: %s", err.Error())
		return errors.Wrap(err, "Failed to select head")
	}

	var out []types.TipSet
	cur := syncHead.Key()
	for {
		logSyncer.Infof("Bootstrap Sync requesting Tipset: %s and %d parents", cur.String(), BootstrapFetchWindow)
		// NB: this will break in the event that we are fetching past the genesis block
		// e.g. bootstrapping a chain of length 8 with a Window set to 10
		tips, err := syncer.fetcher.FetchTipSets(ctx, cur, BootstrapFetchWindow)
		if err != nil {
			logSyncer.Errorf("Failed to fetch tipset at %s. Error: %s", cur.String(), err.Error())
			return errors.Wrap(err, "Failed to fetch tipset")
		}
		out = append(out, tips...)

		// Have we reached their genesis block yet?
		h, err := out[len(out)-1].Height()
		if h == 0 {
			break
		}

		cur, err = out[len(out)-1].Parents()
		if err != nil {
			panic(err)
			return err
		}
	}

	// get the genesis block and order the list
	out = reverse(out)
	genesis := out[0]

	// moment of truth
	// this comparison has a bit of an odor to it.
	if !genesis.Key().Equals(types.NewTipSetKey(syncer.chainStore.GenesisCid())) {
		panic("ohh no wrong chain")
	}

	parent := genesis
	// Try adding the tipsets of the chain to the store, checking for new
	// heaviest tipsets.
	for i, ts := range out {
		// TODO: this "i==0" leaks EC specifics into syncer abstraction
		// for the sake of efficiency, consider plugging up this leak.
		if i == 0 {
			wts, err := syncer.widen(ctx, ts)
			if err != nil {
				return errors.Wrap(err, "failed to widen tipset")
			}
			if wts.Defined() {
				logSyncer.Debug("attempt to sync after widen")
				err = syncer.syncOne(ctx, parent, wts)
				if err != nil {
					return errors.Wrap(err, "failed to sync widened tipset")
				}
			}
		}
		if err = syncer.syncOne(ctx, parent, ts); err != nil {
			// While `syncOne` can indeed fail for reasons other than consensus,
			// adding to the badTipSets at this point is the simplest, since we
			// have access to the chain. If syncOne fails for non-consensus reasons,
			// there is no assumption that the running node's data is valid at all,
			// so we don't really lose anything with this simplification.
			syncer.badTipSets.AddChain(out[i:])
			return errors.Wrap(err, "failed to syncOne")
		}
		if i%500 == 0 {
			logSyncer.Infof("processing block %d of %v for chain with head at %v", i, len(out), syncHead.String())
		}
		parent = ts
	}

	// horay we got the right chain, due to the way bitswap works it has already
	// writen all this data to a store. We may move on to validating it.
	head := out[len(out)-1]
	logSyncer.Infof("Bootstrap Sync Complete new head: %s", head.String())
	syncer.syncMode = CaughtUp
	/*
		if err := syncer.chainStore.SetHead(ctx, head); err != nil {
			return err
		}
	*/

	return nil
}

func (syncer *Syncer) SyncCaughtUp(ctx context.Context, tsKey types.TipSetKey) error {
	logSyncer.Infof("Begin fetch and sync of chain with head %v", tsKey)

	// see if we have done this one before
	if syncer.chainStore.HasTipSetAndState(ctx, tsKey.String()) {
		return nil
	}

	// Walk the chain given by the input blocks back to a known tipset in
	// the store. This is the only code that may go to the network to
	// resolve cids to blocks.
	chain, err := syncer.collectChain(ctx, tsKey)
	if err != nil {
		return err
	}
	parentCids, err := chain[0].Parents()
	if err != nil {
		return err
	}
	parent, err := syncer.chainStore.GetTipSet(parentCids)
	if err != nil {
		return err
	}
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
			if wts.Defined() {
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
		if i%500 == 0 {
			logSyncer.Infof("processing block %d of %v for chain with head at %v", i, len(chain), tsKey.String())
		}
		parent = ts
	}
	return nil

}

func (syncer *Syncer) exceedsFinalityLimit(chain []types.TipSet) bool {
	if len(chain) == 0 {
		return false
	}
	blockHeight, _ := syncer.chainStore.BlockHeight()
	finalityHeight := types.NewBlockHeight(blockHeight).Add(types.NewBlockHeight(uint64(FinalityLimit)))
	chainHeight, _ := chain[0].Height()
	return types.NewBlockHeight(chainHeight).LessThan(finalityHeight)
}

func reverse(tips []types.TipSet) []types.TipSet {
	out := make([]types.TipSet, len(tips))
	for i := 0; i < len(tips); i++ {
		out[i] = tips[len(tips)-(i+1)]
	}
	return out

}
