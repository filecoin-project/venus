package submodule

import (
	"context"

	storageminerconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/storage_miner_connector"

	"github.com/filecoin-project/go-sectorbuilder"
	"github.com/filecoin-project/go-storage-miner"
	"github.com/ipfs/go-datastore"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// StorageMiningSubmodule enhances the `Node` with storage mining capabilities.
type StorageMiningSubmodule struct {
	// StorageMining is used by the miner to fill and seal sectors.
	PieceManager piecemanager.PieceManager

	// PoStGenerator generates election PoSts
	PoStGenerator postgenerator.PoStGenerator

	started   bool
	startFunc func(ctx context.Context) error
	stopFunc  func(ctx context.Context) error
}

// NewStorageMiningSubmodule creates a new storage mining submodule.
func NewStorageMiningSubmodule(minerAddr, workerAddr address.Address, ds datastore.Batching, s sectorbuilder.Interface, c *ChainSubmodule, m *MessagingSubmodule, mw *msg.Waiter, w *WalletSubmodule) (*StorageMiningSubmodule, error) {
	minerNode := storageminerconnector.NewStorageMinerNodeConnector(minerAddr, workerAddr, c.ChainReader, c.State, m.Outbox, mw, w.Wallet)
	storageMiner, err := storage.NewMiner(minerNode, ds, s)
	if err != nil {
		return nil, err
	}

	smbe := piecemanager.NewStorageMinerBackEnd(storageMiner, s)
	sbbe := postgenerator.NewSectorBuilderBackEnd(s)

	modu := &StorageMiningSubmodule{
		PieceManager:  smbe,
		PoStGenerator: sbbe,
	}

	modu.startFunc = func(ctx context.Context) error {
		if !modu.started {
			minerNode.StartHeightListener(ctx, c.HeaviestTipSetCh)
			err := storageMiner.Start(ctx)
			if err != nil {
				return err
			}

			modu.started = true

			return nil
		}

		return nil
	}

	modu.stopFunc = func(ctx context.Context) error {
		if modu.started {
			minerNode.StopHeightListener()
			err := storageMiner.Stop(ctx)
			if err != nil {
				return err
			}

			modu.started = false

			return nil
		}

		return nil
	}

	return modu, nil
}

// Start starts the StorageMiningSubmodule
func (s *StorageMiningSubmodule) Start(ctx context.Context) error {
	return s.startFunc(ctx)
}

// Stop stops the StorageMiningSubmodule
func (s *StorageMiningSubmodule) Stop(ctx context.Context) error {
	return s.stopFunc(ctx)
}
