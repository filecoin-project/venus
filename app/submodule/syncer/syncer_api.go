package syncer

import (
	"context"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chainsync/status"
	xerrors "github.com/pkg/errors"
)

type SyncerAPI struct { //nolint
	syncer *SyncerSubmodule
}

// SyncerStatus returns the current status of the active or last active chain sync operation.
func (syncerAPI *SyncerAPI) SyncerStatus() status.Status {
	return syncerAPI.syncer.SyncProvider.Status()
}

func (syncerAPI *SyncerAPI) ChainTipSetWeight(ctx context.Context, tsk block.TipSetKey) (fbig.Int, error) {
	ts, err := syncerAPI.syncer.ChainModule.ChainReader.GetTipSet(tsk)
	if err != nil {
		return fbig.Int{}, err
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
	parent, err := chainModule.ChainReader.GetBlock(blk.Header.Parents.At(0))
	if err != nil {
		return xerrors.Errorf("loading parent block: %w", err)
	}

	if err := syncerAPI.syncer.SlashFilter.MinedBlock(blk.Header, parent.Height); err != nil {
		log.Errorf("<!!> SLASH FILTER ERROR: %s", err)
		return xerrors.Errorf("<!!> SLASH FILTER ERROR: %w", err)
	}

	// TODO: should we have some sort of fast path to adding a local block?
	bmsgs, err := chainModule.MessageStore.LoadUnsinedMessagesFromCids(blk.BlsMessages)
	if err != nil {
		return xerrors.Errorf("failed to load bls messages: %w", err)
	}
	smsgs, err := chainModule.MessageStore.LoadSignedMessagesFromCids(blk.SecpkMessages)
	if err != nil {
		return xerrors.Errorf("failed to load secpk message: %w", err)
	}

	fb := &block.FullBlock{
		Header:       blk.Header,
		BLSMessages:  bmsgs,
		SECPMessages: smsgs,
	}

	if err := syncerAPI.syncer.Consensus.ValidateMsgMeta(fb); err != nil {
		return xerrors.Errorf("provided messages did not match block: %w", err)
	}

	ts, err := block.NewTipSet(blk.Header)
	if err != nil {
		return xerrors.Errorf("somehow failed to make a tipset out of a single block: %w", err)
	}

	localPeer := syncerAPI.syncer.NetworkModule.Network.GetPeerID()
	if err := syncerAPI.syncer.SyncProvider.HandleNewTipSet(&block.ChainInfo{
		Source: localPeer,
		Sender: localPeer,
		Head:   ts.Key(),
		Height: ts.EnsureHeight(),
	}); err != nil {
		return xerrors.Errorf("sync to submitted block failed: %w", err)
	}

	b, err := blk.Serialize()
	if err != nil {
		return xerrors.Errorf("serializing block for pubsub publishing failed: %w", err)
	}

	return syncerAPI.syncer.BlockTopic.Publish(ctx, b) //nolint:staticcheck
}
