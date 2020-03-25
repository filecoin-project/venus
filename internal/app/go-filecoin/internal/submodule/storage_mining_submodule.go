package submodule

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-sectorbuilder"
	"github.com/filecoin-project/go-storage-miner"
	"github.com/filecoin-project/go-storage-miner/policies/precommit"
	"github.com/filecoin-project/go-storage-miner/policies/selfdeal"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-datastore"

	storageminerconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/storage_miner"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	"github.com/filecoin-project/go-filecoin/internal/pkg/poster"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
)

// StorageMiningSubmodule enhances the `Node` with storage mining capabilities.
type StorageMiningSubmodule struct {
	started bool

	// StorageMining is used by the miner to fill and seal sectors.
	PieceManager piecemanager.PieceManager

	// PoStGenerator generates election PoSts
	PoStGenerator postgenerator.PoStGenerator

	minerNode        *storageminerconnector.StorageMinerNodeConnector
	storageMiner     *storage.Miner
	heaviestTipSetCh chan interface{}
	poster           *poster.Poster
}

// NewStorageMiningSubmodule creates a new storage mining submodule.
func NewStorageMiningSubmodule(minerAddr address.Address, ds datastore.Batching, s sectorbuilder.Interface, c *ChainSubmodule,
	m *MessagingSubmodule, mw *msg.Waiter, w *WalletSubmodule, stateViewer *appstate.Viewer, postGen postgenerator.PoStGenerator) (*StorageMiningSubmodule, error) {
	minerNode := storageminerconnector.NewStorageMinerNodeConnector(minerAddr, c.ChainReader, c.State, m.Outbox, mw, w.Signer, stateViewer)

	// The amount of epochs we expect the storage miner to take to replicate and
	// prove a sector. This value should be shared with the storage miner side
	// of go-fil-markets. The protocol specifies a maximum sealing duration (1)
	// which could be used to improve the accuracy of provingDelay.
	//
	// 1: https://github.com/filecoin-project/specs-actors/commit/fa20d55a3ff0c0134b130dc27850998ffd432580#diff-5a14038af5531003ed825ab608d0dd51R21
	//
	// TODO: What is the correct value for proving delay given 32GiB sectors?
	provingDelay := abi.ChainEpoch(2 * 60 * 24)

	// The quantity of epochs during which the self-deal will be valid.
	selfDealDuration := abi.ChainEpoch(2 * 60 * 24)

	sdp := selfdeal.NewBasicPolicy(minerNode, provingDelay, selfDealDuration)

	// If a sector contains no pieces, this policy will set the sector
	// pre-commit expiration to the current epoch + the provided value. If the
	// sector does contain deals' pieces, the sector pre-commit expiration will
	// be set to the farthest-into-the-future deal end-epoch.
	pcp := precommit.NewBasicPolicy(minerNode, abi.ChainEpoch(2*60*24))

	storageMiner, err := storage.NewMiner(minerNode, ds, s, minerAddr, &sdp, &pcp)
	if err != nil {
		return nil, err
	}

	smbe := piecemanager.NewStorageMinerBackEnd(storageMiner, s)
	if postGen == nil {
		postGen = postgenerator.NewSectorBuilderBackEnd(s)
	}

	modu := &StorageMiningSubmodule{
		PieceManager:     smbe,
		PoStGenerator:    postGen,
		minerNode:        minerNode,
		storageMiner:     storageMiner,
		heaviestTipSetCh: make(chan interface{}),
		poster:           poster.NewPoster(minerAddr, m.Outbox, s, c.State, stateViewer, mw),
	}

	return modu, nil
}

// Start starts the StorageMiningSubmodule
func (s *StorageMiningSubmodule) Start(ctx context.Context) error {
	if s.started {
		return nil
	}

	s.minerNode.StartHeightListener(ctx, s.heaviestTipSetCh)
	err := s.storageMiner.Run(ctx)
	if err != nil {
		return err
	}

	s.started = true

	return nil
}

// Stop stops the StorageMiningSubmodule
func (s *StorageMiningSubmodule) Stop(ctx context.Context) error {
	if !s.started {
		return nil
	}

	s.minerNode.StopHeightListener()
	err := s.storageMiner.Stop(ctx)
	if err != nil {
		return err
	}

	s.poster.StopPoSting()

	s.started = false

	return nil
}

// HandleNewHead submits a new chain head for possible fallback PoSt.
func (s *StorageMiningSubmodule) HandleNewHead(ctx context.Context, newHead block.TipSet) error {
	if !s.started {
		return nil
	}

	s.heaviestTipSetCh <- newHead

	return s.poster.HandleNewHead(ctx, newHead)
}
