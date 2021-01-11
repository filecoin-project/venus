package syncer

import (
	"context"
	"fmt"
	bstore "github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	cbor "github.com/ipfs/go-ipld-cbor"
	"sync"
	"time"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"github.com/prometheus/common/log"
	"go.opencensus.io/trace"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/chainsync/exchange"
	"github.com/filecoin-project/venus/pkg/chainsync/status"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/metrics"
	"github.com/filecoin-project/venus/pkg/metrics/tracing"
	"github.com/filecoin-project/venus/pkg/specactors/policy"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
)

// Syncer updates its chain.Store according to the methods of its
// consensus.Protocol.  It uses a bad tipset cache and a limit on new
// blocks to traverse during chain collection.  The Syncer can query the
// network for blocks.  The Syncer maintains the following invariant on
// its bsstore: all tipsets that pass the syncer's validity checks are added to the
// chain bsstore along with their state root CID.
//
// Ideally the code that syncs the chain according to consensus rules should
// be independent of any particular implementation of consensus.  Currently the
// Syncer is coupled to details of Expected Consensus. This dependence
// exists in the widen function, the fact that widen is called on only one
// tipset in the incoming chain, and assumptions regarding the existence of
// grandparent state in the bsstore.

var ErrForkTooLong = fmt.Errorf("fork longer than threshold")

type Syncer struct {
	// fetcher is the networked block fetching service for fetching blocks
	// and messages.
	fetcher        Fetcher
	exchangeClient exchange.Client
	// BadTipSetCache is used to filter out collections of invalid blocks.
	badTipSets *BadTipSetCache

	// Evaluates tipset messages and stores the resulting states.
	fullValidator FullBlockValidator
	// Validates headers and message structure
	blockValidator BlockValidator
	// Selects the heaviest of two chains
	chainSelector ChainSelector
	// Provides and stores validated tipsets and their state roots.
	chainStore ChainReaderWriter
	// Provides message collections given cids
	messageProvider messageStore

	clock clock.Clock
	// staged is the heaviest tipset seen by the syncer so far
	staged *block.TipSet

	// faultDetector is used to manage information about potential consensus faults
	faultDetector

	// Reporter is used by the syncer to update the current status of the chain.
	reporter status.Reporter

	bsstore    blockstore.Blockstore
	checkPoint block.TipSetKey

	fork fork.IFork
}

// Fetcher defines an interface that may be used to fetch data from the network.
type Fetcher interface {
	// FetchTipSets will only fetch TipSets that evaluate to `false` when passed to `done`,
	// this includes the provided `ts`. The TipSet that evaluates to true when
	// passed to `done` will be in the returned slice. The returns slice of TipSets is in Traversal order.
	FetchTipSets(context.Context, block.TipSetKey, peer.ID, func(*block.TipSet) (bool, error)) ([]*block.TipSet, error)

	// FetchTipSetHeaders will fetch only the headers of tipset blocks.
	// Returned slice in reversal order
	FetchTipSetHeaders(context.Context, block.TipSetKey, peer.ID, func(*block.TipSet) (bool, error)) ([]*block.TipSet, error)
}

// ChainReaderWriter reads and writes the chain bsstore.
type ChainReaderWriter interface {
	GetHead() *block.TipSet
	GetTipSet(block.TipSetKey) (*block.TipSet, error)
	GetTipSetStateRoot(*block.TipSet) (cid.Cid, error)
	GetTipSetReceiptsRoot(*block.TipSet) (cid.Cid, error)
	HasTipSetAndState(context.Context, *block.TipSet) bool
	PutTipSetMetadata(context.Context, *chain.TipSetMetadata) error
	SetHead(context.Context, *block.TipSet) error
	HasSiblingState(*block.TipSet) bool
	GetSiblingState(*block.TipSet) ([]*chain.TipSetMetadata, error)
	GetLatestBeaconEntry(*block.TipSet) (*block.BeaconEntry, error)
	GetGenesisBlock(context.Context) (*block.Block, error)
}

type messageStore interface {
	LoadTipSetMessage(ctx context.Context, ts *block.TipSet) ([]block.BlockMessagesInfo, error)
	LoadMetaMessages(context.Context, cid.Cid) ([]*types.SignedMessage, []*types.UnsignedMessage, error)
	LoadReceipts(context.Context, cid.Cid) ([]types.MessageReceipt, error)
	StoreReceipts(context.Context, []types.MessageReceipt) (cid.Cid, error)
}

// ChainSelector chooses the heaviest between chains.
type ChainSelector interface {
	// IsHeavier returns true if tipset a is heavier than tipset b and false if
	// tipset b is heavier than tipset a.
	IsHeavier(ctx context.Context, a, b *block.TipSet) (bool, error)
	// Weight returns the weight of a tipset after the upgrade to version 1
	Weight(ctx context.Context, ts *block.TipSet) (big.Int, error)
}

// BlockValidator does semanitc validation on headers
type BlockValidator interface {
	// ValidateHeaderSemantic validates conditions on a block header that can be
	// checked with the parent header but not parent state.
	ValidateHeaderSemantic(context.Context, *block.Block, *block.TipSet) error
	// ValidateMessagesSemantic validates a block's messages against parent state without applying the messages
	ValidateMessagesSemantic(context.Context, *block.Block, *block.TipSet) error
}

// FullBlockValidator does semantic validation on fullblocks.
type FullBlockValidator interface {
	// RunStateTransition returns the state root CID resulting from applying the input ts to the
	// prior `stateRoot`.  It returns an error if the transition is invalid.
	RunStateTransition(ctx context.Context, ts *block.TipSet, parentStateRoot cid.Cid) (root cid.Cid, receipts []types.MessageReceipt, err error)
	// Todo add by force
	ValidateMining(ctx context.Context, parent, ts *block.TipSet, parentWeight big.Int, parentReceiptRoot cid.Cid) error
}

// faultDetector tracks data for detecting consensus faults and emits faults
// upon detection.
type faultDetector interface {
	CheckBlock(b *block.Block, p *block.TipSet) error
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
	// ErrUnexpectedStoreState indicates that the syncer's chain bsstore is violating expected invariants.
	ErrUnexpectedStoreState = errors.New("the chain bsstore is in an unexpected state")
)

var syncOneTimer *metrics.Float64Timer

func init() {
	syncOneTimer = metrics.NewTimerMs("syncer/sync_one", "Duration of single tipset validation in milliseconds")
}

var logSyncer = logging.Logger("chainsync.syncer")

// NewSyncer constructs a Syncer ready for use.  The chain reader must have a
// head tipset to initialize the staging field.
func NewSyncer(fv FullBlockValidator,
	hv BlockValidator,
	cs ChainSelector,
	s ChainReaderWriter,
	m messageStore,
	bsstore blockstore.Blockstore,
	f Fetcher,
	exchangeClient exchange.Client,
	sr status.Reporter,
	c clock.Clock,
	fd faultDetector,
	fork fork.IFork) (*Syncer, error) {
	return &Syncer{
		fetcher:        f,
		exchangeClient: exchangeClient,
		badTipSets: &BadTipSetCache{
			bad: make(map[string]struct{}),
		},
		fullValidator:   fv,
		blockValidator:  hv,
		chainSelector:   cs,
		bsstore:         bsstore,
		chainStore:      s,
		messageProvider: m,
		clock:           c,
		faultDetector:   fd,
		reporter:        sr,
		fork:            fork,
	}, nil
}

// InitStaged reads the head from the syncer's chain bsstore and sets the syncer's
// staged field.  Used for initializing syncer.
func (syncer *Syncer) InitStaged() error {
	syncer.staged = syncer.chainStore.GetHead()
	return nil
}

// SetStagedHead sets the syncer's internal staged tipset to the chain's head.
func (syncer *Syncer) SetStagedHead(ctx context.Context) error {
	return syncer.chainStore.SetHead(ctx, syncer.staged)
}

// fetchAndValidateHeaders fetches headers and runs semantic block validation
// on the chain of fetched headers
func (syncer *Syncer) fetchAndValidateHeaders(ctx context.Context, ci *block.ChainInfo) ([]*block.TipSet, error) {
	head := syncer.chainStore.GetHead()
	headHeight, err := head.Height()
	if err != nil {
		return nil, err
	}
	logSyncer.Infof("current head: %v %s", headHeight, head.Key())

	headers, err := syncer.fetcher.FetchTipSetHeaders(ctx, ci.Head, ci.Sender, func(t *block.TipSet) (bool, error) {
		h, err := t.Height()
		if err != nil {
			return true, err
		}

		if h+policy.ChainFinality < headHeight {
			return true, ErrNewChainTooLong
		}

		parentKey, err := t.Parents()
		if err != nil {
			return true, err
		}
		pTs, err := syncer.chainStore.GetTipSet(parentKey)
		if err != nil {
			return true, err
		}
		return syncer.chainStore.HasTipSetAndState(ctx, pTs), nil
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
			err = syncer.blockValidator.ValidateHeaderSemantic(ctx, ts.At(i), parent)
			if err != nil {
				return nil, err
			}
		}
		parent = headers[i]
	}

	return headers, nil
}

// syncOne syncs a single tipset with the chain bsstore. syncOne calculates the
// parent state of the tipset and calls into consensus to run a state transition
// in order to validate the tipset.  In the case the input tipset is valid,
// syncOne calls into consensus to check its weight, and then updates the head
// of the bsstore if this tipset is the heaviest.
//
// Precondition: the caller of syncOne must hold the syncer's lock (syncer.mu) to
// ensure head is not modified by another goroutine during run.
func (syncer *Syncer) syncOne(ctx context.Context, parent, next *block.TipSet) error {
	logSyncer.Infof("Start updated bsstore with %s", next.String())
	priorHeadKey := syncer.chainStore.GetHead()

	// if tipset is already priorHeadKey, we've been here before. do nothing.
	if priorHeadKey.Equals(next) {
		return nil
	}

	stopwatch := syncOneTimer.Start(ctx)
	defer stopwatch.Stop(ctx)

	// Lookup parent state and receipt root. It is guaranteed by the syncer that it is in the chainStore.
	parentStateRoot, err := syncer.chainStore.GetTipSetStateRoot(parent)
	if err != nil {
		return xerrors.Errorf("get parent tipset state failed %w", err)
	}

	if !parent.Key().Equals(syncer.checkPoint) {
		//skip check if just checkpoint
		// validate pre block
		parentWeight, err := syncer.chainSelector.Weight(ctx, parent)
		if err != nil {
			return xerrors.Errorf("calc parent weight failed %w", err)
		}

		parentReceiptRoot, err := syncer.chainStore.GetTipSetReceiptsRoot(parent)
		if err != nil {
			return xerrors.Errorf("get parent tipset receipt failed %w", err)
		}

		err = syncer.fullValidator.ValidateMining(ctx, parent, next, parentWeight, parentReceiptRoot)
		if err != nil {
			return xerrors.Errorf("validate mining failed %w", err)
		}
	}
	// Run a state transition to validate the tipset and compute
	// a new state to add to the bsstore.
	toProcessTime := time.Now()
	root, receipts, err := syncer.fullValidator.RunStateTransition(ctx, next, parentStateRoot)
	if err != nil {
		return xerrors.Errorf("calc current tipset %s state failed %w", next.Key().String(), err)
	}

	for i := 0; i < next.Len(); i++ {
		err = syncer.faultDetector.CheckBlock(next.At(i), parent)
		if err != nil {
			return err
		}
	}

	receiptCid, err := syncer.messageProvider.StoreReceipts(ctx, receipts)
	if err != nil {
		return errors.Wrapf(err, "could not bsstore message rerceipts for tip set %s", next.String())
	}

	logSyncer.Infow("Process Block ", "Height:", next.EnsureHeight(), " Root:", root, " receiptcid ", receiptCid, " time: ", time.Now().Sub(toProcessTime).Milliseconds())

	err = syncer.chainStore.PutTipSetMetadata(ctx, &chain.TipSetMetadata{
		TipSet:          next,
		TipSetStateRoot: root,
		TipSetReceipts:  receiptCid,
	})
	if err != nil {
		return err
	}
	logSyncer.Infof("Successfully updated bsstore with %s", next.String())
	return nil
}

// ancestorsFromStore returns the parent and grandparent tipsets of `ts`
func (syncer *Syncer) ancestorsFromStore(ts *block.TipSet) (*block.TipSet, *block.TipSet, error) {
	parentCids, err := ts.Parents()
	if err != nil {
		return nil, nil, err
	}
	parent, err := syncer.chainStore.GetTipSet(parentCids)
	if err != nil {
		return nil, nil, err
	}

	if parent.EnsureHeight() == 0 {
		// parent == genesis ==>
		return parent, nil, nil
	}

	grandParentCids, err := parent.Parents()
	if err != nil {
		return nil, nil, err
	}
	if grandParentCids.IsEmpty() { //todo genesis have parents ?
		// parent == genesis ==> grandParent undef
		return parent, nil, nil
	}
	grandParent, err := syncer.chainStore.GetTipSet(grandParentCids)
	if err != nil {
		return nil, nil, err
	}
	return parent, grandParent, nil
}

func (syncer *Syncer) logReorg(ctx context.Context, curHead, newHead *block.TipSet) {
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

// widen computes a tipset implied by the input tipset and the bsstore that
// could potentially be the heaviest tipset. In the context of EC, widen
// returns the union of the input tipset and the biggest tipset with the same
// parents from the bsstore.
// TODO: this leaks EC abstractions into the syncer, we should think about this.
func (syncer *Syncer) widen(ctx context.Context, ts *block.TipSet) (*block.TipSet, error) {
	// Lookup tipsets with the same parents from the bsstore.
	if !syncer.chainStore.HasSiblingState(ts) {
		return nil, nil
	}
	//find current tipset base the same parent and height
	candidates, err := syncer.chainStore.GetSiblingState(ts)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	// Only take the tipset with the most blocks (this is EC specific logic) todo ?
	max := candidates[0].TipSet
	for _, candidate := range candidates[0:] {
		if candidate.TipSet.Len() > max.Len() {
			max = candidate.TipSet
		}
	}

	// Form a new tipset from the union of ts and the largest in the bsstore, de-duped.
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
		return nil, err
	}

	// check that the tipset is distinct from the input and tipsets from the bsstore.
	if wts.String() == ts.String() || wts.String() == max.String() {
		return nil, nil
	}

	return wts, nil
}

// HandleNewTipSet validates and syncs the chain rooted at the provided tipset
// to a chain bsstore.  Iff catchup is false then the syncer will set the head.
func (syncer *Syncer) HandleNewTipSet(ctx context.Context, ci *block.ChainInfo, catchup bool) error {
	logSyncer.Infof("HandleNewTipSet height: %v, catchup: %v", ci.Height, catchup)
	for {
		if syncer.staged != nil {
			break
		}
		time.Sleep(time.Second * 2)
	}

	err := syncer.handleNewTipSet(ctx, ci)
	if err != nil {
		return err
	}
	if catchup {
		return nil
	}
	return syncer.SetStagedHead(ctx)
}

var beInSyncing bool

// handleNewTipSet extends the Syncer's chain bsstore with the given tipset if
// the chain is a valid extension.  It stages new heaviest tipsets for later
// setting the chain head
func (syncer *Syncer) handleNewTipSet(ctx context.Context, ci *block.ChainInfo) (err error) {
	logSyncer.Infof("Begin fetch and sync of chain with head %v from %s at height %v", ci.Head, ci.Sender.String(), ci.Height)
	ctx, span := trace.StartSpan(ctx, "Syncer.HandleNewTipSet")
	span.AddAttributes(trace.StringAttribute("tipset", ci.Head.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	if beInSyncing {
		return
	}
	beInSyncing = true
	defer func() {
		beInSyncing = false //reset to start new sync
	}()

	//If the store already has this tipset then the syncer is finished.
	//if syncer.chainStore.HasTipSetAndState(ctx, ci.Head) {
	//	return nil
	//}
	if ci.Head.Equals(syncer.staged.Key()) {
		return nil
	}

	syncer.reporter.UpdateStatus(status.SyncingStarted(syncer.clock.Now().Unix()), status.SyncHead(ci.Head), status.SyncHeight(ci.Height), status.SyncComplete(false))
	defer syncer.reporter.UpdateStatus(status.SyncComplete(true))

	syncer.reporter.UpdateStatus(func(s *status.Status) {
		s.FetchingHead = ci.Head
		s.FetchingHeight = ci.Height
	})

	tipsets, err := syncer.fetchChainBlocks(ctx, syncer.staged, ci.Head)
	if err != nil {
		return errors.Wrapf(err, "failure fetching or validating headers")
	}
	//todo ci shuold give a tipset to ignore block for each blocks
	//todo incoming tipset should compare with current sync head to ignore sasme
	if tipsets[len(tipsets)-1].At(0).ParentWeight.LessThan(syncer.staged.At(0).ParentWeight) {
		return xerrors.New("do not sync a less weight tip")
	}

	logSyncer.Infof("fetch & validate header success at %v %s ...", tipsets[0].EnsureHeight(), tipsets[0].Key())
	errProcessChan := make(chan error, 1)
	errProcessChan <- nil //init
	var wg sync.WaitGroup
	//todo  write a pipline segment processor function
	err = SegProcess(tipsets, func(segTipset []*block.TipSet) error {
		// fetch messages
		startTip := segTipset[0].EnsureHeight()
		emdTipset := segTipset[len(segTipset)-1].EnsureHeight()
		logSyncer.Infof("start to fetch message segement %d-%d", startTip, emdTipset)
		_, err := syncer.fetchSegMessage(ctx, segTipset)
		if err != nil {
			return err
		}
		logSyncer.Infof("finish to fetch message segement %d-%d", startTip, emdTipset)
		err = <-errProcessChan
		if err != nil {
			return xerrors.Errorf("process message failed %v", err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			logSyncer.Infof("start to process message segement %d-%d", startTip, emdTipset)
			defer logSyncer.Infof("finish to process message segement %d-%d", startTip, emdTipset)
			errProcess := syncer.processTipSetSeg(ctx, segTipset)
			if errProcess != nil {
				errProcessChan <- errProcess
				return
			}
			errProcessChan <- syncer.SetStagedHead(ctx)
		}()

		return nil
	})

	if err != nil {
		return err
	}
	wg.Wait()
	select {
	case err = <-errProcessChan:
		return err
	default:
		return nil
	}
}

func (syncer *Syncer) fetchChainBlocks(ctx context.Context, knownTip *block.TipSet, targetTip block.TipSetKey) ([]*block.TipSet, error) {
	var chainTipsets []*block.TipSet

	var flushDb = func(saveTips []*block.TipSet) error {
		bs := bstore.NewTemporary()
		cborStore := cbor.NewCborStore(bs)
		for _, tips := range saveTips {
			for _, blk := range tips.Blocks() {
				_, err := cborStore.Put(ctx, blk)
				if err != nil {
					return err
				}
			}
		}
		return blockstoreutil.CopyBlockstore(ctx, bs, syncer.bsstore)
	}

	var windows = 500
	untilHeight := knownTip.EnsureHeight()
	count := 0
loop:
	for len(chainTipsets) == 0 || chainTipsets[len(chainTipsets)-1].EnsureHeight() > untilHeight {
		tipset, err := syncer.chainStore.GetTipSet(targetTip)
		if err == nil {
			chainTipsets = append(chainTipsets, tipset)
			targetTip = tipset.EnsureParents()
			count++
			if count%500 == 0 {
				logSyncer.Info("load from local db ", "Height: ", tipset.EnsureHeight())
			}
			continue
		}

		fetchHeaders, err := syncer.exchangeClient.GetBlocks(ctx, targetTip, windows)
		if err != nil {
			return nil, err
		}

		if len(fetchHeaders) == 0 {
			break loop
		}

		logSyncer.Infof("fetch  blocks %d height from %d-%d", len(fetchHeaders), fetchHeaders[0].EnsureHeight(), fetchHeaders[len(fetchHeaders)-1].EnsureHeight())
		if err = flushDb(fetchHeaders); err != nil {
			return nil, err
		}
		for _, b := range fetchHeaders {
			if b.EnsureHeight() < untilHeight {
				break loop
			}
			chainTipsets = append(chainTipsets, b)
			targetTip = b.EnsureParents()
		}
	}

	if len(chainTipsets) == 0 {
		return nil, xerrors.Errorf("sync chain store has no tipset %s", targetTip.String())
	}

	base := chainTipsets[len(chainTipsets)-1]
	if base.Equals(knownTip) {
		chainTipsets = chainTipsets[:len(chainTipsets)-1]
		base = chainTipsets[len(chainTipsets)-1]
	}

	if base.IsChildOf(knownTip) {
		// common case: receiving blocks that are building on top of our best tipset
		chain.Reverse(chainTipsets)
		return chainTipsets, nil
	}

	knownParent, err := syncer.chainStore.GetTipSet(knownTip.EnsureParents())
	if err != nil {
		return nil, xerrors.Errorf("failed to load next local tipset: %w", err)
	}
	if base.IsChildOf(knownParent) {
		// common case: receiving a block thats potentially part of the same tipset as our best block
		chain.Reverse(chainTipsets)
		return chainTipsets, nil
	}

	log.Warnf("(fork detected) synced header chain")
	fork, err := syncer.syncFork(ctx, base, knownTip)
	if err != nil {
		if xerrors.Is(err, ErrForkTooLong) {
			// TODO: we're marking this block bad in the same way that we mark invalid blocks bad. Maybe distinguish?
			log.Warn("adding forked chain to our bad tipset cache")
			/*		for _, b := range incoming.Blocks() {
					syncer.bad.Add(b.Cid(), NewBadBlockReason(incoming.Cids(), "fork past finality"))
				}*/
		}
		return nil, xerrors.Errorf("failed to sync fork: %w", err)
	}
	err = flushDb(fork)
	if err != nil {
		return nil, err
	}
	chainTipsets = append(chainTipsets, fork...)
	chain.Reverse(chainTipsets)
	return chainTipsets, nil
}

func (syncer *Syncer) syncFork(ctx context.Context, incoming *block.TipSet, known *block.TipSet) ([]*block.TipSet, error) {
	// TODO: Does this mean we always ask for ForkLengthThreshold blocks from the network, even if we just need, like, 2?
	// Would it not be better to ask in smaller chunks, given that an ~ForkLengthThreshold is very rare?
	tips, err := syncer.exchangeClient.GetBlocks(ctx, incoming.EnsureParents(), int(policy.ChainFinality))
	if err != nil {
		return nil, err
	}

	gensisiBlock, err := syncer.chainStore.GetGenesisBlock(ctx)
	if err != nil {
		return nil, err
	}

	nts, err := syncer.chainStore.GetTipSet(known.EnsureParents())
	if err != nil {
		return nil, xerrors.Errorf("failed to load next local tipset: %w", err)
	}

	for cur := 0; cur < len(tips); {
		if nts.EnsureHeight() == 0 {
			if !gensisiBlock.Equals(nts.At(0)) {
				return nil, xerrors.Errorf("somehow synced chain that linked back to a different genesis (bad genesis: %s)", nts.Key())
			}
			return nil, xerrors.Errorf("synced chain forked at genesis, refusing to sync; incoming: %s", incoming.ToSlice())
		}

		if nts.Equals(tips[cur]) {
			return tips[:cur], nil
		}

		if nts.EnsureHeight() < tips[cur].EnsureHeight() {
			cur++
		} else {
			nts, err = syncer.chainStore.GetTipSet(nts.EnsureParents())
			if err != nil {
				return nil, xerrors.Errorf("loading next local tipset: %w", err)
			}
		}
	}

	return nil, ErrForkTooLong
}

func (syncer *Syncer) fetchSegMessage(ctx context.Context, segTipset []*block.TipSet) ([]*block.FullTipSet, error) {
	//get message from local bsstore
	if len(segTipset) == 0 {
		return []*block.FullTipSet{}, nil
	}

	chain.Reverse(segTipset)
	defer chain.Reverse(segTipset)

	fullTipSets := make([]*block.FullTipSet, len(segTipset))
	defer block.ReverseFullBlock(fullTipSets)

	var leftChain []*block.TipSet
	var letFullChain []*block.FullTipSet
	for index, tip := range segTipset {
		fullTipset, err := syncer.getFullBlock(ctx, tip)
		if err != nil {
			leftChain = segTipset[index:]
			letFullChain = fullTipSets[index:]
			break
		}
		fullTipSets[index] = fullTipset
	}

	if len(leftChain) == 0 {
		return fullTipSets, nil
	}
	// fetch message from remote nodes
	bs := bstore.NewTemporary()
	cborStore := cbor.NewCborStore(bs)

	messages, err := syncer.exchangeClient.GetChainMessages(ctx, leftChain)
	if err != nil {
		return nil, err
	}

	for index, tip := range leftChain {
		fts, err := zipTipSetAndMessages(bs, tip, messages[index].Bls, messages[index].Secpk, messages[index].BlsIncludes, messages[index].SecpkIncludes)
		if err != nil {
			return nil, xerrors.Errorf("message processing failed: %w", err)
		}
		letFullChain[index] = fts

		//save message
		for _, m := range messages[index].Bls {
			if _, err := cborStore.Put(ctx, m); err != nil {
				return nil, xerrors.Errorf("BLS message processing failed: %w", err)
			}
		}

		for _, m := range messages[index].Secpk {
			if _, err := cborStore.Put(ctx, m); err != nil {
				return nil, xerrors.Errorf("SECP message processing failed: %w", err)
			}
		}
	}

	err = blockstoreutil.CopyBlockstore(ctx, bs, syncer.bsstore)
	if err != nil {
		return nil, errors.Wrapf(err, "failure fetching full blocks")
	}
	return fullTipSets, nil
}

func (syncer *Syncer) getFullBlock(ctx context.Context, tipset *block.TipSet) (*block.FullTipSet, error) {
	fullBlocks := make([]*block.FullBlock, tipset.Len())
	for index, blk := range tipset.Blocks() {
		secpMsg, blsMsg, err := syncer.messageProvider.LoadMetaMessages(ctx, blk.Messages)
		if err != nil {
			return nil, err
		}
		fullBlocks[index] = &block.FullBlock{
			Header:       blk,
			BLSMessages:  blsMsg,
			SECPMessages: secpMsg,
		}
	}
	return block.NewFullTipSet(fullBlocks), nil
}

func (syncer *Syncer) processTipSetSeg(ctx context.Context, segTipset []*block.TipSet) error {
	//todo fork
	parent, _, err := syncer.ancestorsFromStore(segTipset[0])
	if err != nil {
		return err
	}
	for i, ts := range segTipset {
		// TODO: this "i==0" leaks EC specifics into syncer abstraction
		// for the sake of efficiency, consider plugging up this leak.
		var wts *block.TipSet
		if i == 0 {
			//merge new block into current tipset in bsstore
			wts, err = syncer.widen(ctx, ts)
			if err != nil {
				return xerrors.Errorf("widen tipset %d error %w", ts.EnsureHeight(), err)
			}
			if wts.Defined() {
				logSyncer.Info("attempt to sync after widen")
				err = syncer.syncOne(ctx, parent, wts)
				if err != nil {
					return err
				}
				if !wts.Key().Equals(syncer.checkPoint) {
					err = syncer.stageIfHeaviest(ctx, wts)
					if err != nil {
						return err
					}
				}
			}
		}
		// If the segTipset has length greater than 1, then we need to sync each tipset
		// in the chain in order to process the chain fully, including the non-widened
		// first tipset.
		// If the chan has length == 1, we can avoid processing the non-widened tipset
		// as a performance optimization, because this tipset cannot be heavier
		// than the widened first tipset.
		if !wts.Defined() || len(segTipset) > 1 {
			err = syncer.syncOne(ctx, parent, ts)
			if err != nil {
				// While `syncOne` can indeed fail for reasons other than consensus,
				// adding to the badTipSets at this point is the simplest, since we
				// have access to the chain. If syncOne fails for non-consensus reasons,
				// there is no assumption that the running node's data is valid at all,
				// so we don't really lose anything with this simplification.
				syncer.badTipSets.AddChain(segTipset[i:])
				return errors.Wrapf(err, "failed to sync tipset %s, number %d of %d in chain", ts.Key(), i, len(segTipset))
			}

			if !ts.Key().Equals(syncer.checkPoint) {
				err = syncer.stageIfHeaviest(ctx, ts)
				if err != nil {
					return err
				}
			}
		}
		parent = ts
	}
	err = syncer.SetStagedHead(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (syncer *Syncer) stageIfHeaviest(ctx context.Context, candidate *block.TipSet) error {
	// stageIfHeaviest sets the provided candidates to the staging head of the chain if they
	// are heavier. Precondtion: candidates are validated and added to the store.
	heavier, err := syncer.chainSelector.IsHeavier(ctx, candidate, syncer.staged)
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

// TODO: this function effectively accepts unchecked input from the network,
// either validate it here, or ensure that its validated elsewhere (maybe make
// sure the blocksync code checks it?)
// maybe this code should actually live in blocksync??
func zipTipSetAndMessages(bs blockstore.Blockstore, ts *block.TipSet, allbmsgs []*types.UnsignedMessage, allsmsgs []*types.SignedMessage, bmi, smi [][]uint64) (*block.FullTipSet, error) {
	if len(ts.Blocks()) != len(smi) || len(ts.Blocks()) != len(bmi) {
		return nil, fmt.Errorf("msgincl length didnt match tipset size")
	}

	fts := &block.FullTipSet{}
	for bi, b := range ts.Blocks() {
		if msgc := len(bmi[bi]) + len(smi[bi]); msgc > constants.BlockMessageLimit {
			return nil, fmt.Errorf("block %q has too many messages (%d)", b.Cid(), msgc)
		}

		var smsgs []*types.SignedMessage
		var smsgCids []cid.Cid
		for _, m := range smi[bi] {
			smsgs = append(smsgs, allsmsgs[m])
			mCid, _ := allsmsgs[m].Cid()
			smsgCids = append(smsgCids, mCid)
		}

		var bmsgs []*types.UnsignedMessage
		var bmsgCids []cid.Cid
		for _, m := range bmi[bi] {
			bmsgs = append(bmsgs, allbmsgs[m])
			mCid, _ := allbmsgs[m].Cid()
			bmsgCids = append(bmsgCids, mCid)
		}

		mrcid, err := chain.ComputeMsgMeta(bs, bmsgCids, smsgCids)
		if err != nil {
			return nil, err
		}

		if b.Messages != mrcid {
			return nil, fmt.Errorf("messages didnt match message root in header for ts %s", ts.Key())
		}

		fb := &block.FullBlock{
			Header:       b,
			BLSMessages:  bmsgs,
			SECPMessages: smsgs,
		}

		fts.Blocks = append(fts.Blocks, fb)
	}

	return fts, nil
}

const maxProcessLen = 32

func SegProcess(ts []*block.TipSet, cb func(ts []*block.TipSet) error) (err error) {
	for {
		if len(ts) == 0 {
			break
		} else if len(ts) < maxProcessLen {
			// break out if less than process len
			err = cb(ts)
			break
		} else {
			processTs := ts[0:maxProcessLen]
			err = cb(processTs)
			if err != nil {
				break
			}
			ts = ts[maxProcessLen:]
		}
	}

	logSyncer.Infof("Sync Process End,Remaining: %v, err: %v ...", len(ts), err)

	return err
}
