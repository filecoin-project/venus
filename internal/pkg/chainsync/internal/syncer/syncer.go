package syncer

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync/status"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

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
	// fetcher is the networked block fetching service for fetching blocks
	// and messages.
	fetcher Fetcher
	// BadTipSetCache is used to filter out collections of invalid blocks.
	badTipSets *BadTipSetCache

	// Evaluates tipset messages and stores the resulting states.
	fullValidator FullBlockValidator
	// Validates headers
	headerValidator HeaderValidator
	// Selects the heaviest of two chains
	chainSelector ChainSelector
	// Provides and stores validated tipsets and their state roots.
	chainStore ChainReaderWriter
	// Provides message collections given cids
	messageProvider messageStore

	clock clock.Clock
	// staged is the heaviest tipset seen by the syncer so far
	staged block.TipSet

	// faultDetector is used to manage information about potential consensus faults
	faultDetector

	// Reporter is used by the syncer to update the current status of the chain.
	reporter status.Reporter
}

// Fetcher defines an interface that may be used to fetch data from the network.
type Fetcher interface {
	// FetchTipSets will only fetch TipSets that evaluate to `false` when passed to `done`,
	// this includes the provided `ts`. The TipSet that evaluates to true when
	// passed to `done` will be in the returned slice. The returns slice of TipSets is in Traversal order.
	FetchTipSets(context.Context, block.TipSetKey, peer.ID, func(block.TipSet) (bool, error)) ([]block.TipSet, error)

	// FetchTipSetHeaders will fetch only the headers of tipset blocks.
	// Returned slice in reversal order
	FetchTipSetHeaders(context.Context, block.TipSetKey, peer.ID, func(block.TipSet) (bool, error)) ([]block.TipSet, error)
}

// ChainReaderWriter reads and writes the chain store.
type ChainReaderWriter interface {
	GetHead() block.TipSetKey
	GetTipSet(tsKey block.TipSetKey) (block.TipSet, error)
	GetTipSetStateRoot(tsKey block.TipSetKey) (cid.Cid, error)
	GetTipSetReceiptsRoot(tsKey block.TipSetKey) (cid.Cid, error)
	HasTipSetAndState(ctx context.Context, tsKey block.TipSetKey) bool
	PutTipSetMetadata(ctx context.Context, tsas *chain.TipSetMetadata) error
	SetHead(ctx context.Context, ts block.TipSet) error
	HasTipSetAndStatesWithParentsAndHeight(pTsKey block.TipSetKey, h uint64) bool
	GetTipSetAndStatesByParentsAndHeight(pTsKey block.TipSetKey, h uint64) ([]*chain.TipSetMetadata, error)
}

type messageStore interface {
	LoadMessages(context.Context, types.TxMeta) ([]*types.SignedMessage, []*types.UnsignedMessage, error)
	LoadReceipts(context.Context, cid.Cid) ([]*types.MessageReceipt, error)
	StoreReceipts(context.Context, []*types.MessageReceipt) (cid.Cid, error)
}

// ChainSelector chooses the heaviest between chains.
type ChainSelector interface {
	// IsHeavier returns true if tipset a is heavier than tipset b and false if
	// tipset b is heavier than tipset a.
	IsHeavier(ctx context.Context, a, b block.TipSet, aStateID, bStateID cid.Cid) (bool, error)
	// Weight returns the weight of a tipset after the upgrade to version 1
	Weight(ctx context.Context, ts block.TipSet, stRoot cid.Cid) (uint64, error)
}

// HeaderValidator does semanitc validation on headers
type HeaderValidator interface {
	// ValidateSemantic validates conditions on a block header that can be
	// checked with the parent header but not parent state.
	ValidateSemantic(ctx context.Context, header *block.Block, parents block.TipSet) error
}

// FullBlockValidator does semantic validation on fullblocks.
type FullBlockValidator interface {
	// RunStateTransition returns the state root CID resulting from applying the input ts to the
	// prior `stateRoot`.  It returns an error if the transition is invalid.
	RunStateTransition(ctx context.Context, ts block.TipSet, blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.SignedMessage, ancestors []block.TipSet, parentWeight uint64, stateID cid.Cid, receiptRoot cid.Cid) (cid.Cid, []*types.MessageReceipt, error)
}

// faultDetector tracks data for detecting consensus faults and emits faults
// upon detection.
type faultDetector interface {
	CheckBlock(b *block.Block, p block.TipSet) error
}

var reorgCnt *metrics.Int64Counter

func init() {
	reorgCnt = metrics.NewInt64Counter("chain/reorg_count", "The number of reorgs that have occurred.")
}

var (
	// ErrChainHasBadTipSet is returned when the syncer traverses a chain with a cached bad tipset.
	ErrChainHasBadTipSet = errors.New("input chain contains a cached bad tipset")
	// ErrNewChainTooLong is returned when processing a fork that split off from the main chain too many blocks ago.
	ErrNewChainTooLong = errors.New("input chain forked from best chain past finality limit")
	// ErrUnexpectedStoreState indicates that the syncer's chain store is violating expected invariants.
	ErrUnexpectedStoreState = errors.New("the chain store is in an unexpected state")
)

var syncOneTimer *metrics.Float64Timer

func init() {
	syncOneTimer = metrics.NewTimerMs("syncer/sync_one", "Duration of single tipset validation in milliseconds")
}

var logSyncer = logging.Logger("chainsync.syncer")

// NewSyncer constructs a Syncer ready for use.  The chain reader must have a
// head tipset to initialize the staging field.
func NewSyncer(fv FullBlockValidator, hv HeaderValidator, cs ChainSelector, s ChainReaderWriter, m messageStore, f Fetcher, sr status.Reporter, c clock.Clock, fd faultDetector) (*Syncer, error) {
	return &Syncer{
		fetcher: f,
		badTipSets: &BadTipSetCache{
			bad: make(map[string]struct{}),
		},
		fullValidator:   fv,
		headerValidator: hv,
		chainSelector:   cs,
		chainStore:      s,
		messageProvider: m,
		clock:           c,
		faultDetector:   fd,
		reporter:        sr,
	}, nil
}

// InitStaged reads the head from the syncer's chain store and sets the syncer's
// staged field.  Used for initializing syncer.
func (syncer *Syncer) InitStaged() error {
	staged, err := syncer.chainStore.GetTipSet(syncer.chainStore.GetHead())
	if err != nil {
		return err
	}
	syncer.staged = staged
	return nil
}

// SetStagedHead sets the syncer's internal staged tipset to the chain's head.
func (syncer *Syncer) SetStagedHead(ctx context.Context) error {
	return syncer.chainStore.SetHead(ctx, syncer.staged)
}

// fetchAndValidateHeaders fetches headers and runs semantic block validation
// on the chain of fetched headers
func (syncer *Syncer) fetchAndValidateHeaders(ctx context.Context, ci *block.ChainInfo) ([]block.TipSet, error) {
	head, err := syncer.chainStore.GetTipSet(syncer.chainStore.GetHead())
	if err != nil {
		return nil, err
	}
	headHeight, err := head.Height()
	if err != nil {
		return nil, err
	}
	headers, err := syncer.fetcher.FetchTipSetHeaders(ctx, ci.Head, ci.Sender, func(t block.TipSet) (bool, error) {
		h, err := t.Height()
		if err != nil {
			return true, err
		}
		if h+consensus.FinalityEpochs < headHeight {
			return true, ErrNewChainTooLong
		}

		parents, err := t.Parents()
		if err != nil {
			return true, err
		}
		return syncer.chainStore.HasTipSetAndState(ctx, parents), nil
	})
	if err != nil {
		return nil, err
	}
	// Fetcher returns chain in Traversal order, reverse it to height order
	chain.Reverse(headers)

	parent, _, err := syncer.ancestorsFromStore(headers[0])
	if err != nil {
		return nil, err
	}
	for i, ts := range headers {
		for i := 0; i < ts.Len(); i++ {
			err = syncer.headerValidator.ValidateSemantic(ctx, ts.At(i), parent)
			if err != nil {
				return nil, err
			}
		}
		parent = headers[i]
	}
	return headers, nil
}

// syncOne syncs a single tipset with the chain store. syncOne calculates the
// parent state of the tipset and calls into consensus to run a state transition
// in order to validate the tipset.  In the case the input tipset is valid,
// syncOne calls into consensus to check its weight, and then updates the head
// of the store if this tipset is the heaviest.
//
// Precondition: the caller of syncOne must hold the syncer's lock (syncer.mu) to
// ensure head is not modified by another goroutine during run.
func (syncer *Syncer) syncOne(ctx context.Context, grandParent, parent, next block.TipSet) error {
	priorHeadKey := syncer.chainStore.GetHead()

	// if tipset is already priorHeadKey, we've been here before. do nothing.
	if priorHeadKey.Equals(next.Key()) {
		return nil
	}

	stopwatch := syncOneTimer.Start(ctx)
	defer stopwatch.Stop(ctx)

	// Lookup parent state and receipt root. It is guaranteed by the syncer that it is in the chainStore.
	stateRoot, err := syncer.chainStore.GetTipSetStateRoot(parent.Key())
	if err != nil {
		return err
	}

	// Gather ancestor chain needed to process state transition.
	h, err := next.Height()
	if err != nil {
		return err
	}
	ancestorHeight := types.NewBlockHeight(h).Sub(types.NewBlockHeight(uint64(consensus.AncestorRoundsNeeded)))
	ancestors, err := chain.GetRecentAncestors(ctx, parent, syncer.chainStore, ancestorHeight)
	if err != nil {
		return err
	}

	// Gather tipset messages
	var nextSecpMessages [][]*types.SignedMessage
	var nextBlsMessages [][]*types.UnsignedMessage
	for i := 0; i < next.Len(); i++ {
		blk := next.At(i)
		secpMsgs, blsMsgs, err := syncer.messageProvider.LoadMessages(ctx, blk.Messages)
		if err != nil {
			return errors.Wrapf(err, "syncing tip %s failed loading message list %s for block %s", next.Key(), blk.Messages, blk.Cid())
		}

		nextBlsMessages = append(nextBlsMessages, blsMsgs)
		nextSecpMessages = append(nextSecpMessages, secpMsgs)
	}

	// Gather validated parent weight
	parentWeight, err := syncer.calculateParentWeight(ctx, parent, grandParent)
	if err != nil {
		return err
	}

	parentReceiptRoot, err := syncer.chainStore.GetTipSetReceiptsRoot(parent.Key())
	if err != nil {
		return err
	}

	// Run a state transition to validate the tipset and compute
	// a new state to add to the store.
	root, receipts, err := syncer.fullValidator.RunStateTransition(ctx, next, nextBlsMessages, nextSecpMessages, ancestors, parentWeight, stateRoot, parentReceiptRoot)
	if err != nil {
		return err
	}

	// Now that the tipset is validated preconditions are satisfied to check
	// consensus faults
	for i := 0; i < next.Len(); i++ {
		err := syncer.faultDetector.CheckBlock(next.At(i), parent)
		if err != nil {
			return err
		}
	}

	receiptCid, err := syncer.messageProvider.StoreReceipts(ctx, receipts)
	if err != nil {
		return errors.Wrapf(err, "could not store message rerceipts for tip set %s", next.String())
	}

	err = syncer.chainStore.PutTipSetMetadata(ctx, &chain.TipSetMetadata{
		TipSet:          next,
		TipSetStateRoot: root,
		TipSetReceipts:  receiptCid,
	})
	if err != nil {
		return err
	}
	logSyncer.Debugf("Successfully updated store with %s", next.String())
	return nil
}

// TODO #3537 this should be stored the first time it is computed and retrieved
// from disk just like aggregate state roots.
func (syncer *Syncer) calculateParentWeight(ctx context.Context, parent, grandParent block.TipSet) (uint64, error) {
	if grandParent.Equals(block.UndefTipSet) {
		return syncer.chainSelector.Weight(ctx, parent, cid.Undef)
	}
	gpStRoot, err := syncer.chainStore.GetTipSetStateRoot(grandParent.Key())
	if err != nil {
		return 0, err
	}
	return syncer.chainSelector.Weight(ctx, parent, gpStRoot)
}

// ancestorsFromStore returns the parent and grandparent tipsets of `ts`
func (syncer *Syncer) ancestorsFromStore(ts block.TipSet) (block.TipSet, block.TipSet, error) {
	parentCids, err := ts.Parents()
	if err != nil {
		return block.UndefTipSet, block.UndefTipSet, err
	}
	parent, err := syncer.chainStore.GetTipSet(parentCids)
	if err != nil {
		return block.UndefTipSet, block.UndefTipSet, err
	}
	grandParentCids, err := parent.Parents()
	if err != nil {
		return block.UndefTipSet, block.UndefTipSet, err
	}
	if grandParentCids.Empty() {
		// parent == genesis ==> grandParent undef
		return parent, block.UndefTipSet, nil
	}
	grandParent, err := syncer.chainStore.GetTipSet(grandParentCids)
	if err != nil {
		return block.UndefTipSet, block.UndefTipSet, err
	}
	return parent, grandParent, nil
}

func (syncer *Syncer) logReorg(ctx context.Context, curHead, newHead block.TipSet) {
	curHeadIter := chain.IterAncestors(ctx, syncer.chainStore, curHead)
	newHeadIter := chain.IterAncestors(ctx, syncer.chainStore, newHead)
	commonAncestor, err := chain.FindCommonAncestor(curHeadIter, newHeadIter)
	if err != nil {
		// Should never get here because reorgs should always have a
		// common ancestor..
		logSyncer.Warnf("unexpected error when running FindCommonAncestor for reorg log: %s", err.Error())
		return
	}

	reorg := chain.IsReorg(curHead, newHead, commonAncestor)
	if reorg {
		reorgCnt.Inc(ctx, 1)
		dropped, added, err := chain.ReorgDiff(curHead, newHead, commonAncestor)
		if err == nil {
			logSyncer.With(
				"currentHead", curHead,
				"newHead", newHead,
			).Infof("reorg dropping %d height and adding %d", dropped, added)
		} else {
			logSyncer.With(
				"currentHead", curHead,
				"newHead", newHead,
			).Infof("reorg")
			logSyncer.Errorw("unexpected error from ReorgDiff during log", "error", err)
		}
	}
}

// widen computes a tipset implied by the input tipset and the store that
// could potentially be the heaviest tipset. In the context of EC, widen
// returns the union of the input tipset and the biggest tipset with the same
// parents from the store.
// TODO: this leaks EC abstractions into the syncer, we should think about this.
func (syncer *Syncer) widen(ctx context.Context, ts block.TipSet) (block.TipSet, error) {
	// Lookup tipsets with the same parents from the store.
	parentSet, err := ts.Parents()
	if err != nil {
		return block.UndefTipSet, err
	}
	height, err := ts.Height()
	if err != nil {
		return block.UndefTipSet, err
	}
	if !syncer.chainStore.HasTipSetAndStatesWithParentsAndHeight(parentSet, height) {
		return block.UndefTipSet, nil
	}
	candidates, err := syncer.chainStore.GetTipSetAndStatesByParentsAndHeight(parentSet, height)
	if err != nil {
		return block.UndefTipSet, err
	}
	if len(candidates) == 0 {
		return block.UndefTipSet, nil
	}

	// Only take the tipset with the most blocks (this is EC specific logic)
	max := candidates[0].TipSet
	for _, candidate := range candidates[0:] {
		if candidate.TipSet.Len() > max.Len() {
			max = candidate.TipSet
		}
	}

	// Form a new tipset from the union of ts and the largest in the store, de-duped.
	var blockSlice []*block.Block
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
	wts, err := block.NewTipSet(blockSlice...)
	if err != nil {
		return block.UndefTipSet, err
	}

	// check that the tipset is distinct from the input and tipsets from the store.
	if wts.String() == ts.String() || wts.String() == max.String() {
		return block.UndefTipSet, nil
	}

	return wts, nil
}

// HandleNewTipSet validates and syncs the chain rooted at the provided tipset
// to a chain store.  Iff catchup is false then the syncer will set the head.
func (syncer *Syncer) HandleNewTipSet(ctx context.Context, ci *block.ChainInfo, catchup bool) error {
	err := syncer.handleNewTipSet(ctx, ci)
	if err != nil {
		return err
	}
	if catchup {
		return nil
	}
	return syncer.SetStagedHead(ctx)
}

func (syncer *Syncer) handleNewTipSet(ctx context.Context, ci *block.ChainInfo) (err error) {
	// handleNewTipSet extends the Syncer's chain store with the given tipset if
	// the chain is a valid extension.  It stages new heaviest tipsets for later
	// setting the chain head
	logSyncer.Debugf("Begin fetch and sync of chain with head %v", ci.Head)
	ctx, span := trace.StartSpan(ctx, "Syncer.HandleNewTipSet")
	span.AddAttributes(trace.StringAttribute("tipset", ci.Head.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	// If the store already has this tipset then the syncer is finished.
	if syncer.chainStore.HasTipSetAndState(ctx, ci.Head) {
		return nil
	}

	syncer.reporter.UpdateStatus(status.SyncingStarted(syncer.clock.Now().Unix()), status.SyncHead(ci.Head), status.SyncHeight(ci.Height), status.SyncComplete(false))
	defer syncer.reporter.UpdateStatus(status.SyncComplete(true))
	syncer.reporter.UpdateStatus(status.SyncFetchComplete(false))

	tipsets, err := syncer.fetchAndValidateHeaders(ctx, ci)
	if err != nil {
		return err
	}

	// Once headers check out, fetch messages
	_, err = syncer.fetcher.FetchTipSets(ctx, ci.Head, ci.Sender, func(t block.TipSet) (bool, error) {
		parents, err := t.Parents()
		if err != nil {
			return true, err
		}
		height, err := t.Height()
		if err != nil {
			return false, err
		}

		// update status with latest fetched head and height
		syncer.reporter.UpdateStatus(status.FetchHead(t.Key()), status.FetchHeight(height))
		return syncer.chainStore.HasTipSetAndState(ctx, parents), nil
	})
	if err != nil {
		return err
	}

	syncer.reporter.UpdateStatus(status.SyncFetchComplete(true))
	if err != nil {
		return err
	}

	parent, grandParent, err := syncer.ancestorsFromStore(tipsets[0])
	if err != nil {
		return err
	}

	// Try adding the tipsets of the chain to the store, checking for new
	// heaviest tipsets.
	for i, ts := range tipsets {
		// TODO: this "i==0" leaks EC specifics into syncer abstraction
		// for the sake of efficiency, consider plugging up this leak.
		var wts block.TipSet
		if i == 0 {
			wts, err = syncer.widen(ctx, ts)
			if err != nil {
				return err
			}
			if wts.Defined() {
				logSyncer.Debug("attempt to sync after widen")
				err = syncer.syncOne(ctx, grandParent, parent, wts)
				if err != nil {
					return err
				}
				err = syncer.stageIfHeaviest(ctx, wts)
				if err != nil {
					return err
				}
			}
		}
		// If the tipsets has length greater than 1, then we need to sync each tipset
		// in the chain in order to process the chain fully, including the non-widened
		// first tipset.
		// If the chan has length == 1, we can avoid processing the non-widened tipset
		// as a performance optimization, because this tipset cannot be heavier
		// than the widened first tipset.
		if !wts.Defined() || len(tipsets) > 1 {
			err = syncer.syncOne(ctx, grandParent, parent, ts)
			if err != nil {
				// While `syncOne` can indeed fail for reasons other than consensus,
				// adding to the badTipSets at this point is the simplest, since we
				// have access to the chain. If syncOne fails for non-consensus reasons,
				// there is no assumption that the running node's data is valid at all,
				// so we don't really lose anything with this simplification.
				syncer.badTipSets.AddChain(tipsets[i:])
				return err
			}
		}

		if i%500 == 0 {
			logSyncer.Infof("processing block %d of %v for chain with head at %v", i, len(tipsets), ci.Head.String())
		}
		grandParent = parent
		parent = ts
	}
	return syncer.stageIfHeaviest(ctx, parent)
}

func (syncer *Syncer) stageIfHeaviest(ctx context.Context, candidate block.TipSet) error {
	// stageIfHeaviest sets the provided candidates to the staging head of the chain if they
	// are heavier. Precondtion: candidates are validated and added to the store.
	parentKey, err := candidate.Parents()
	if err != nil {
		return err
	}
	candidateParentStateID, err := syncer.chainStore.GetTipSetStateRoot(parentKey)
	if err != nil {
		return err
	}

	stagedParentKey, err := syncer.staged.Parents()
	if err != nil {
		return err
	}
	var stagedParentStateID cid.Cid
	if !stagedParentKey.Empty() { // head is not genesis
		stagedParentStateID, err = syncer.chainStore.GetTipSetStateRoot(stagedParentKey)
		if err != nil {
			return err
		}
	}

	heavier, err := syncer.chainSelector.IsHeavier(ctx, candidate, syncer.staged, candidateParentStateID, stagedParentStateID)
	if err != nil {
		return err
	}

	// If it is the heaviest update the chainStore.
	if heavier {
		// Gather the entire new chain for reorg comparison and logging.
		syncer.logReorg(ctx, syncer.staged, candidate)
		syncer.staged = candidate
	}

	return nil
}

// Status returns the current syncer status.
func (syncer *Syncer) Status() status.Status {
	return syncer.reporter.Status()
}
