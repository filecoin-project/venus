package syncer

import (
	"context"
	"time"

	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	syncTypes "github.com/filecoin-project/venus/pkg/chainsync/types"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/pkg/types"
	logging "github.com/ipfs/go-log/v2"
	xerrors "github.com/pkg/errors"
)

var syncAPILog = logging.Logger("syncAPI")

var _ apiface.ISyncer = &syncerAPI{}

type syncerAPI struct { //nolint
	syncer *SyncerSubmodule
}

// SyncerStatus returns the current status of the active or last active chain sync operation.
func (sa *syncerAPI) SyncerTracker(ctx context.Context) *syncTypes.TargetTracker {
	return sa.syncer.ChainSyncManager.BlockProposer().SyncTracker()
}

// SyncerStatus returns the current status of the active or last active chain sync operation.
func (sa *syncerAPI) SetConcurrent(ctx context.Context, concurrent int64) error {
	sa.syncer.ChainSyncManager.BlockProposer().SetConcurrent(concurrent)
	return nil
}

// SyncerStatus returns the current status of the active or last active chain sync operation.
func (sa *syncerAPI) Concurrent(ctx context.Context) int64 {
	return sa.syncer.ChainSyncManager.BlockProposer().Concurrent()
}

// ChainTipSetWeight computes weight for the specified tipset.
func (sa *syncerAPI) ChainTipSetWeight(ctx context.Context, tsk types.TipSetKey) (big.Int, error) {
	ts, err := sa.syncer.ChainModule.ChainReader.GetTipSet(tsk)
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
	parent, err := chainModule.ChainReader.GetBlock(ctx, blk.Header.Parents.Cids()[0])
	if err != nil {
		return xerrors.Errorf("loading parent block: %v", err)
	}

	if err := sa.syncer.SlashFilter.MinedBlock(blk.Header, parent.Height); err != nil {
		log.Errorf("<!!> SLASH FILTER ERROR: %s", err)
		return xerrors.Errorf("<!!> SLASH FILTER ERROR: %v", err)
	}

	// TODO: should we have some sort of fast path to adding a local block?
	bmsgs, err := chainModule.MessageStore.LoadUnsignedMessagesFromCids(blk.BlsMessages)
	if err != nil {
		return xerrors.Errorf("failed to load bls messages: %v", err)
	}
	smsgs, err := chainModule.MessageStore.LoadSignedMessagesFromCids(blk.SecpkMessages)
	if err != nil {
		return xerrors.Errorf("failed to load secpk message: %v", err)
	}

	fb := &types.FullBlock{
		Header:       blk.Header,
		BLSMessages:  bmsgs,
		SECPMessages: smsgs,
	}

	if err := sa.syncer.BlockValidator.ValidateMsgMeta(fb); err != nil {
		return xerrors.Errorf("provided messages did not match block: %v", err)
	}

	ts, err := types.NewTipSet(blk.Header)
	if err != nil {
		return xerrors.Errorf("somehow failed to make a tipset out of a single block: %v", err)
	}

	if _, err := chainModule.ChainReader.PutObject(ctx, blk.Header); err != nil {
		return err
	}
	localPeer := sa.syncer.NetworkModule.Network.GetPeerID()
	ci := types.NewChainInfo(localPeer, localPeer, ts)
	if err := sa.syncer.SyncProvider.HandleNewTipSet(ci); err != nil {
		return xerrors.Errorf("sync to submitted block failed: %v", err)
	}

	b, err := blk.Serialize()
	if err != nil {
		return xerrors.Errorf("serializing block for pubsub publishing failed: %v", err)
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
func (sa *syncerAPI) StateCall(ctx context.Context, msg *types.UnsignedMessage, tsk types.TipSetKey) (*apitypes.InvocResult, error) {
	start := time.Now()
	ts, err := sa.syncer.ChainModule.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}
	ret, err := sa.syncer.Consensus.Call(ctx, msg, ts)
	if err != nil {
		return nil, err
	}
	duration := time.Since(start)

	mcid := msg.Cid()
	return &apitypes.InvocResult{
		MsgCid:         mcid,
		Msg:            msg,
		MsgRct:         &ret.Receipt,
		ExecutionTrace: types.ExecutionTrace{},
		Duration:       duration,
	}, nil
}

//SyncState just compatible code lotus
func (sa *syncerAPI) SyncState(ctx context.Context) (*apitypes.SyncState, error) {
	tracker := sa.syncer.ChainSyncManager.BlockProposer().SyncTracker()
	tracker.History()

	syncState := &apitypes.SyncState{
		VMApplied: 0,
	}

	count := 0
	toActiveSync := func(t *syncTypes.Target) apitypes.ActiveSync {
		currentHeight := t.Base.Height()
		if t.Current != nil {
			currentHeight = t.Current.Height()
		}

		msg := ""
		if t.Err != nil {
			msg = t.Err.Error()
		}
		count++

		activeSync := apitypes.ActiveSync{
			WorkerID: uint64(count),
			Base:     t.Base,
			Target:   t.Head,
			Height:   currentHeight,
			Start:    t.Start,
			End:      t.End,
			Message:  msg,
		}

		switch t.State {
		case syncTypes.StageIdle:
			activeSync.Stage = apitypes.StageIdle
		case syncTypes.StageSyncErrored:
			activeSync.Stage = apitypes.StageSyncErrored
		case syncTypes.StageSyncComplete:
			activeSync.Stage = apitypes.StageSyncComplete
		case syncTypes.StateInSyncing:
			activeSync.Stage = apitypes.StageMessages
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
