package chain

import (
	"context"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/metrics"
	"github.com/filecoin-project/go-filecoin/net"
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
)

type syncerChainReader interface {
	HasTipSetAndState(ctx context.Context, tsKey string) bool
	GetTipSet(tsKey types.TipSetKey) (types.TipSet, error)
	BlockHeight() (uint64, error)
	GenesisCid() cid.Cid
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
	// 1. syncOne assumes that store.Head() does not change when
	// comparing tipset weights and updating the store
	// 2. HandleNewTipset assumes that calls to widen and then syncOne
	// are not run concurrently with other calls to widen to ensure
	// that the syncer always finds the heaviest existing tipset.
	mu sync.Mutex
	// fetcher is the networked block fetching service for fetching blocks
	// and messages.
	fetcher net.Fetcher

	// Evaluates and processes tipset messages and stores the resulting states.
	evaluator Evaluator
	// Provides and stores validated tipsets and their state roots.
	store syncerChainReader

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
func NewSyncer(e Evaluator, s syncerChainReader, f net.Fetcher, pm PeerManager, syncMode SyncMode) *Syncer {
	return &Syncer{
		fetcher:   f,
		evaluator: e,
		store:     s,
		syncMode:  syncMode,
		pm:        pm,
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
		default:
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

// SyncBootstrap performs the bootstrap sync process, if successful the syncer will
// be placed in "CaughtUp" mode, else an error is returned.
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
		// TODO this will need to be rethought when the fether is capable of returning a list of tipsets
		h, err := out[len(out)-1].Height()
		if err != nil {
			return err
		}
		if h == 0 {
			break
		}

		cur, err = out[len(out)-1].Parents()
		if err != nil {
			return err
		}
	}

	// get the genesis block and order the list
	out = reverse(out)
	genesis := out[0]

	// moment of truth
	// this comparison has a bit of an odor to it.
	if !genesis.Key().Equals(types.NewTipSetKey(syncer.store.GenesisCid())) {
		panic("ohh no wrong chain")
	}

	if err := syncer.evaluator.Evaluate(ctx, genesis, out); err != nil {
		return err
	}

	// horay we got the right chain, due to the way bitswap works it has already
	// written all this data to a store. We may move on to validating it.
	logSyncer.Infof("Bootstrap Sync Complete new head: %s", out[len(out)-1].String())
	syncer.syncMode = CaughtUp
	return nil
}

// SyncCaughtUp performs a caught up sync.
func (syncer *Syncer) SyncCaughtUp(ctx context.Context, tsKey types.TipSetKey) error {
	logSyncer.Infof("Begin fetch and sync of chain with head %v", tsKey)

	// see if we have done this one before
	if syncer.store.HasTipSetAndState(ctx, tsKey.String()) {
		return nil
	}

	// see if this is a known bad tipset
	if syncer.evaluator.IsBadTipSet(tsKey) {
		return errors.Errorf("bad tipset: %s", tsKey.String())
	}

	// fetch the full tipset for inspection
	ts, err := syncer.fetchTipSetWithTimeout(ctx, tsKey)
	if err != nil {
		return err
	}

	// is this within the finality limit
	if syncer.evaluator.ExceedsFinalityLimit(ts) {
		return ErrNewChainTooLong
	}

	// this is a tipset we should attempt to catch up to
	// we will start at its parents
	cur, err := ts.Parents()
	if err != nil {
		return err
	}
	chain := []types.TipSet{ts}
	for {
		tip, err := syncer.fetchTipSetWithTimeout(ctx, cur)
		if err != nil {
			return err
		}

		// have we reached a known state?
		if syncer.store.HasTipSetAndState(ctx, tip.String()) {
			break
		}
		// TODO should we be doing any finality checks in this loop?

		chain = append(chain, tip)
		cur, err = tip.Parents()
		if err != nil {
			return err
		}
	}
	chain = reverse(chain)

	parentCids, err := chain[0].Parents()
	if err != nil {
		return err
	}

	parent, err := syncer.store.GetTipSet(parentCids)
	if err != nil {
		return err
	}
	return syncer.evaluator.Evaluate(ctx, parent, chain)
}

func reverse(tips []types.TipSet) []types.TipSet {
	out := make([]types.TipSet, len(tips))
	for i := 0; i < len(tips); i++ {
		out[i] = tips[len(tips)-(i+1)]
	}
	return out

}
