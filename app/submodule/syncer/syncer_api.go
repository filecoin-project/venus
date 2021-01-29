package syncer

import (
	"context"
	syncTypes "github.com/filecoin-project/venus/pkg/chainsync/types"
	"time"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/types"
	logging "github.com/ipfs/go-log/v2"
	xerrors "github.com/pkg/errors"
)

var syncAPILog = logging.Logger("syncAPI")

type SyncerAPI struct { //nolint
	syncer *SyncerSubmodule
}

// SyncerStatus returns the current status of the active or last active chain sync operation.
func (syncerAPI *SyncerAPI) SyncerTracker() *syncTypes.TargetTracker {
	return syncerAPI.syncer.ChainSyncManager.BlockProposer().SyncTracker()
}

// SyncerStatus returns the current status of the active or last active chain sync operation.
func (syncerAPI *SyncerAPI) SetConcurrent(concurrent int64) {
	syncerAPI.syncer.ChainSyncManager.BlockProposer().SetConcurrent(concurrent)
}

func (syncerAPI *SyncerAPI) ChainTipSetWeight(ctx context.Context, tsk block.TipSetKey) (big.Int, error) {
	ts, err := syncerAPI.syncer.ChainModule.ChainReader.GetTipSet(tsk)
	if err != nil {
		return big.Int{}, err
	}
	return syncerAPI.syncer.ChainSelector.Weight(ctx, ts)
}

// ChainSyncHandleNewTipSet submits a chain head to the syncer for processing.
func (syncerAPI *SyncerAPI) ChainSyncHandleNewTipSet(ci *block.ChainInfo) error {
	return syncerAPI.syncer.SyncProvider.HandleNewTipSet(ci)
}

func (syncerAPI *SyncerAPI) SyncSubmitBlock(ctx context.Context, blk *block.BlockMsg) error {
	//todo many dot. how to get directly
	chainModule := syncerAPI.syncer.ChainModule
	parent, err := chainModule.ChainReader.GetBlock(ctx, blk.Header.Parents.Cids()[0])
	if err != nil {
		return xerrors.Errorf("loading parent block: %v", err)
	}

	if err := syncerAPI.syncer.SlashFilter.MinedBlock(blk.Header, parent.Height); err != nil {
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

	fb := &block.FullBlock{
		Header:       blk.Header,
		BLSMessages:  bmsgs,
		SECPMessages: smsgs,
	}

	if err := syncerAPI.syncer.BlockValidator.ValidateMsgMeta(fb); err != nil {
		return xerrors.Errorf("provided messages did not match block: %v", err)
	}

	ts, err := block.NewTipSet(blk.Header)
	if err != nil {
		return xerrors.Errorf("somehow failed to make a tipset out of a single block: %v", err)
	}

	if _, err := chainModule.ChainReader.PutObject(ctx, blk.Header); err != nil {
		return err
	}
	localPeer := syncerAPI.syncer.NetworkModule.Network.GetPeerID()
	ci := block.NewChainInfo(localPeer, localPeer, ts)
	if err := syncerAPI.syncer.SyncProvider.HandleNewTipSet(ci); err != nil {
		return xerrors.Errorf("sync to submitted block failed: %v", err)
	}

	b, err := blk.Serialize()
	if err != nil {
		return xerrors.Errorf("serializing block for pubsub publishing failed: %v", err)
	}
	go func() {
		err = syncerAPI.syncer.BlockTopic.Publish(ctx, b) //nolint:staticcheck
		if err != nil {
			syncAPILog.Warnf("publish failed: %s, %v", blk.Cid(), err)
		}
	}()
	return nil
}

func (syncerAPI *SyncerAPI) StateCall(ctx context.Context, msg *types.UnsignedMessage, tsk block.TipSetKey) (*InvocResult, error) {
	start := time.Now()
	ts, err := syncerAPI.syncer.ChainModule.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}
	ret, err := syncerAPI.syncer.Consensus.Call(ctx, msg, ts)
	if err != nil {
		return nil, err
	}
	duration := time.Now().Sub(start)

	mcid, _ := msg.Cid()
	return &InvocResult{
		MsgCid:         mcid,
		Msg:            msg,
		MsgRct:         &ret.Receipt,
		ExecutionTrace: types.ExecutionTrace{},
		Duration:       duration,
	}, nil
}

//SyncState just compatible code lotus
func (syncerAPI *SyncerAPI) SyncState(ctx context.Context) (*SyncState, error) {
	tracker := syncerAPI.syncer.ChainSyncManager.BlockProposer().SyncTracker()
	tracker.History()

	syncState := &SyncState{
		VMApplied: 0,
	}

	count := 0
	toActiveSync := func(t *syncTypes.Target) ActiveSync {
		currentHeight := t.Base.EnsureHeight()
		if t.Current != nil {
			currentHeight = t.Current.EnsureHeight()
		}

		msg := ""
		if t.Err != nil {
			msg = t.Err.Error()
		}
		count++

		activeSync := ActiveSync{
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
			activeSync.Stage = StageIdle
		case syncTypes.StageSyncErrored:
			activeSync.Stage = StageSyncErrored
		case syncTypes.StageSyncComplete:
			activeSync.Stage = StageSyncComplete
		case syncTypes.StateInSyncing:
			activeSync.Stage = StageMessages
		}

		return activeSync
	}
	//current
	for _, t := range tracker.Buckets() {
		if t.State != syncTypes.StageSyncErrored {
			activeSync := toActiveSync(t)
			syncState.ActiveSyncs = append(syncState.ActiveSyncs, activeSync)
		}
	}
	//history
	for _, t := range tracker.History() {
		if t.State != syncTypes.StageSyncErrored {
			activeSync := toActiveSync(t)
			syncState.ActiveSyncs = append(syncState.ActiveSyncs, activeSync)
		}
	}

	return syncState, nil
}
