package eth

import (
	"context"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/ethhashlookup"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type ethTxHashManager struct {
	chainAPI              v1.IChain
	messageStore          *chain.MessageStore
	forkUpgradeConfig     *config.ForkUpgradeConfig
	TransactionHashLookup *ethhashlookup.EthTxHashLookup
}

func (m *ethTxHashManager) Apply(ctx context.Context, from, to *types.TipSet) error {
	for _, blk := range to.Blocks() {
		blkMsgs, err := m.chainAPI.ChainGetBlockMessages(ctx, blk.Cid())
		if err != nil {
			return err
		}

		for _, smsg := range blkMsgs.SecpkMessages {
			if smsg.Signature.Type != crypto.SigTypeDelegated {
				continue
			}

			hash, err := ethTxHashFromSignedMessage(ctx, smsg, m.chainAPI)
			if err != nil {
				return err
			}

			err = m.TransactionHashLookup.UpsertHash(hash, smsg.Cid())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *ethTxHashManager) Revert(ctx context.Context, from, to *types.TipSet) error {
	return nil
}

func (m *ethTxHashManager) PopulateExistingMappings(ctx context.Context, minHeight abi.ChainEpoch) error {
	if minHeight < m.forkUpgradeConfig.UpgradeHyggeHeight {
		minHeight = m.forkUpgradeConfig.UpgradeHyggeHeight
	}

	ts, err := m.chainAPI.ChainHead(ctx)
	if err != nil {
		return err
	}
	for ts.Height() > minHeight {
		for _, block := range ts.Blocks() {
			msgs, err := m.messageStore.SecpkMessagesForBlock(ctx, block)
			if err != nil {
				// If we can't find the messages, we've either imported from snapshot or pruned the store
				log.Debug("exiting message mapping population at epoch ", ts.Height())
				return nil
			}

			for _, msg := range msgs {
				m.ProcessSignedMessage(ctx, msg)
			}
		}

		var err error
		ts, err = m.chainAPI.ChainGetTipSet(ctx, ts.Parents())
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *ethTxHashManager) ProcessSignedMessage(ctx context.Context, msg *types.SignedMessage) {
	if msg.Signature.Type != crypto.SigTypeDelegated {
		return
	}

	ethTx, err := newEthTxFromSignedMessage(ctx, msg, m.chainAPI)
	if err != nil {
		log.Errorf("error converting filecoin message to eth tx: %s", err)
		return
	}

	err = m.TransactionHashLookup.UpsertHash(ethTx.Hash, msg.Cid())
	if err != nil {
		log.Errorf("error inserting tx mapping to db: %s", err)
		return
	}
}

func waitForMpoolUpdates(ctx context.Context, ch <-chan types.MpoolUpdate, manager *ethTxHashManager) {
	for {
		select {
		case <-ctx.Done():
			return
		case u := <-ch:
			if u.Type != types.MpoolAdd {
				continue
			}

			manager.ProcessSignedMessage(ctx, u.Message)
		}
	}
}

func ethTxHashGC(ctx context.Context, retentionDays int, manager *ethTxHashManager) {
	if retentionDays == 0 {
		return
	}

	gcPeriod := 1 * time.Hour
	for {
		entriesDeleted, err := manager.TransactionHashLookup.DeleteEntriesOlderThan(retentionDays)
		if err != nil {
			log.Errorf("error garbage collecting eth transaction hash database: %s", err)
		}
		log.Info("garbage collection run on eth transaction hash lookup database. %d entries deleted", entriesDeleted)
		time.Sleep(gcPeriod)
	}
}
