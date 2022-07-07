package syncer

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/filecoin-project/go-state-types/big"
	syncTypes "github.com/filecoin-project/venus/pkg/chainsync/types"
	"github.com/filecoin-project/venus/pkg/fvm"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	logging "github.com/ipfs/go-log/v2"
)

var syncAPILog = logging.Logger("syncAPI")

var _ v1api.ISyncer = &syncerAPI{}

type syncerAPI struct { //nolint
	syncer *SyncerSubmodule
}

// SyncerTracker returns the TargetTracker of syncing.
func (sa *syncerAPI) SyncerTracker(ctx context.Context) *types.TargetTracker {
	tracker := sa.syncer.ChainSyncManager.BlockProposer().SyncTracker()
	tt := &types.TargetTracker{
		History: make([]*types.Target, 0),
		Buckets: make([]*types.Target, 0),
	}
	convertTarget := func(src *syncTypes.Target) *types.Target {
		return &types.Target{
			State:     convertSyncStateStage(src.State),
			Base:      src.Base,
			Current:   src.Current,
			Start:     src.Start,
			End:       src.End,
			Err:       src.Err,
			ChainInfo: src.ChainInfo,
		}
	}
	for _, target := range tracker.History() {
		tt.History = append(tt.History, convertTarget(target))
	}
	for _, target := range tracker.Buckets() {
		tt.Buckets = append(tt.Buckets, convertTarget(target))
	}

	return tt
}

func convertSyncStateStage(srtState syncTypes.SyncStateStage) types.SyncStateStage {
	var state types.SyncStateStage
	switch srtState {
	case syncTypes.StageIdle:
		state = types.StageIdle
	case syncTypes.StageSyncErrored:
		state = types.StageSyncErrored
	case syncTypes.StageSyncComplete:
		state = types.StageSyncComplete
	case syncTypes.StateInSyncing:
		state = types.StageMessages
	}

	return state
}

// SetConcurrent set the syncer worker(go-routine) number of chain syncing
func (sa *syncerAPI) SetConcurrent(ctx context.Context, concurrent int64) error {
	sa.syncer.ChainSyncManager.BlockProposer().SetConcurrent(concurrent)
	return nil
}

// Concurrent get the syncer worker(go-routine) number of chain syncing.
func (sa *syncerAPI) Concurrent(ctx context.Context) int64 {
	return sa.syncer.ChainSyncManager.BlockProposer().Concurrent()
}

// ChainTipSetWeight computes weight for the specified tipset.
func (sa *syncerAPI) ChainTipSetWeight(ctx context.Context, tsk types.TipSetKey) (big.Int, error) {
	ts, err := sa.syncer.ChainModule.ChainReader.GetTipSet(ctx, tsk)
	if err != nil {
		return big.Int{}, err
	}
	return sa.syncer.ChainSelector.Weight(ctx, ts)
}

// ChainSyncHandleNewTipSet submits a chain head to the syncer for processing.
func (sa *syncerAPI) ChainSyncHandleNewTipSet(ctx context.Context, ci *types.ChainInfo) error {
	return sa.syncer.SyncProvider.HandleNewTipSet(ci)
}

// SyncSubmitBlock can be used to submit a newly created block to the.
// network through this node
func (sa *syncerAPI) SyncSubmitBlock(ctx context.Context, blk *types.BlockMsg) error {
	//todo many dot. how to get directly
	chainModule := sa.syncer.ChainModule
	parent, err := chainModule.ChainReader.GetBlock(ctx, blk.Header.Parents[0])
	if err != nil {
		return fmt.Errorf("loading parent block: %v", err)
	}

	if err := sa.syncer.SlashFilter.MinedBlock(ctx, blk.Header, parent.Height); err != nil {
		log.Errorf("<!!> SLASH FILTER ERROR: %s", err)
		return fmt.Errorf("<!!> SLASH FILTER ERROR: %v", err)
	}

	// TODO: should we have some sort of fast path to adding a local block?
	bmsgs, err := chainModule.MessageStore.LoadUnsignedMessagesFromCids(ctx, blk.BlsMessages)
	if err != nil {
		return fmt.Errorf("failed to load bls messages: %v", err)
	}
	smsgs, err := chainModule.MessageStore.LoadSignedMessagesFromCids(ctx, blk.SecpkMessages)
	if err != nil {
		return fmt.Errorf("failed to load secpk message: %v", err)
	}

	fb := &types.FullBlock{
		Header:       blk.Header,
		BLSMessages:  bmsgs,
		SECPMessages: smsgs,
	}

	if err := sa.syncer.BlockValidator.ValidateMsgMeta(ctx, fb); err != nil {
		return fmt.Errorf("provided messages did not match block: %v", err)
	}

	ts, err := types.NewTipSet([]*types.BlockHeader{blk.Header})
	if err != nil {
		return fmt.Errorf("somehow failed to make a tipset out of a single block: %v", err)
	}

	if _, err := chainModule.ChainReader.PutObject(ctx, blk.Header); err != nil {
		return err
	}
	localPeer := sa.syncer.NetworkModule.Network.GetPeerID()
	ci := types.NewChainInfo(localPeer, localPeer, ts)
	if err := sa.syncer.SyncProvider.HandleNewTipSet(ci); err != nil {
		return fmt.Errorf("sync to submitted block failed: %v", err)
	}

	b, err := blk.Serialize()
	if err != nil {
		return fmt.Errorf("serializing block for pubsub publishing failed: %v", err)
	}
	go func() {
		err = sa.syncer.BlockTopic.Publish(ctx, b) //nolint:staticcheck
		if err != nil {
			syncAPILog.Warnf("publish failed: %s, %v", blk.Cid(), err)
		}
	}()
	return nil
}

// MethodGroup: State
// The State methods are used to query, inspect, and interact with chain state.
// Most methods take a TipSetKey as a parameter. The state looked up is the parent state of the tipset.
// A nil TipSetKey can be provided as a param, this will cause the heaviest tipset in the chain to be used.

// StateCall runs the given message and returns its result without any persisted changes.
//
// StateCall applies the message to the tipset's parent state. The
// message is not applied on-top-of the messages in the passed-in
// tipset.
func (sa *syncerAPI) StateCall(ctx context.Context, msg *types.Message, tsk types.TipSetKey) (*types.InvocResult, error) {
	start := time.Now()
	ts, err := sa.syncer.ChainModule.ChainReader.GetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("loading tipset %s: %v", tsk, err)
	}
	ret, err := sa.syncer.Stmgr.Call(ctx, msg, ts)
	if err != nil {
		return nil, err
	}
	duration := time.Since(start)

	mcid := msg.Cid()
	return &types.InvocResult{
		MsgCid:         mcid,
		Msg:            msg,
		MsgRct:         &ret.Receipt,
		ExecutionTrace: types.ExecutionTrace{},
		Duration:       duration,
	}, nil
}

//SyncState just compatible code lotus
func (sa *syncerAPI) SyncState(ctx context.Context) (*types.SyncState, error) {
	tracker := sa.syncer.ChainSyncManager.BlockProposer().SyncTracker()
	tracker.History()

	syncState := &types.SyncState{
		VMApplied: atomic.LoadUint64(&fvm.StatApplied),
	}

	count := 0
	toActiveSync := func(t *syncTypes.Target) types.ActiveSync {
		currentHeight := t.Base.Height()
		if t.Current != nil {
			currentHeight = t.Current.Height()
		}

		msg := ""
		if t.Err != nil {
			msg = t.Err.Error()
		}
		count++

		activeSync := types.ActiveSync{
			WorkerID: uint64(count),
			Base:     t.Base,
			Target:   t.Head,
			Stage:    convertSyncStateStage(t.State),
			Height:   currentHeight,
			Start:    t.Start,
			End:      t.End,
			Message:  msg,
		}
		return activeSync
	}
	//current
	for _, t := range tracker.Buckets() {
		if t.State != syncTypes.StageSyncErrored {
			syncState.ActiveSyncs = append(syncState.ActiveSyncs, toActiveSync(t))
		}
	}
	//history
	for _, t := range tracker.History() {
		if t.State != syncTypes.StageSyncErrored {
			syncState.ActiveSyncs = append(syncState.ActiveSyncs, toActiveSync(t))
		}
	}

	return syncState, nil
}
