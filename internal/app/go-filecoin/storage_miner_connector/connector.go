package storageminerconnector

import (
	"context"
	"errors"
	"math"

	"github.com/filecoin-project/go-address"
	commcid "github.com/filecoin-project/go-fil-commcid"
	"github.com/filecoin-project/go-storage-miner"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsampler"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
)

var log = logging.Logger("connector") // nolint: deadcode

// StorageMinerNodeConnector is a struct which satisfies the go-storage-miner
// needs of "the node," e.g. interacting with the blockchain, persisting sector
// states to disk, and so forth.
type StorageMinerNodeConnector struct {
	minerAddr  address.Address
	workerAddr address.Address

	newListener     chan *chainsampler.HeightThresholdListener
	heightListeners []*chainsampler.HeightThresholdListener
	listenerDone    chan struct{}

	chainStore *chain.Store
	chainState *cst.ChainStateReadWriter
	outbox     *message.Outbox
	waiter     *msg.Waiter
	wallet     *wallet.Wallet
}

var _ storage.NodeAPI = new(StorageMinerNodeConnector)

// NewStorageMinerNodeConnector produces a StorageMinerNodeConnector, which adapts
// types in this codebase to the interface representing "the node" which is
// expected by the go-storage-miner project.
func NewStorageMinerNodeConnector(minerAddress address.Address, workerAddress address.Address, chainStore *chain.Store, chainState *cst.ChainStateReadWriter, outbox *message.Outbox, waiter *msg.Waiter, wallet *wallet.Wallet) *StorageMinerNodeConnector {
	return &StorageMinerNodeConnector{
		minerAddr:    minerAddress,
		workerAddr:   workerAddress,
		listenerDone: make(chan struct{}),
		chainStore:   chainStore,
		chainState:   chainState,
		outbox:       outbox,
		waiter:       waiter,
		wallet:       wallet,
	}
}

// StartHeightListener starts the scheduler that manages height listeners.
func (m *StorageMinerNodeConnector) StartHeightListener(ctx context.Context, htc <-chan interface{}) {
	go func() {
		var previousHead block.TipSet
		for {
			select {
			case <-htc:
				head, err := m.handleNewTipSet(ctx, previousHead)
				if err != nil {
					log.Warn("failed to handle new tipset")
				} else {
					previousHead = head
				}
			case heightListener := <-m.newListener:
				m.heightListeners = append(m.heightListeners, heightListener)
			case <-m.listenerDone:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// StopHeightListener stops the scheduler that manages height listeners.
func (m *StorageMinerNodeConnector) StopHeightListener() {
	m.listenerDone <- struct{}{}
}

func (m *StorageMinerNodeConnector) handleNewTipSet(ctx context.Context, previousHead block.TipSet) (block.TipSet, error) {
	newHeadKey := m.chainStore.GetHead()
	newHead, err := m.chainStore.GetTipSet(newHeadKey)
	if err != nil {
		return block.TipSet{}, err
	}

	_, newTips, err := chain.CollectTipsToCommonAncestor(ctx, m.chainStore, previousHead, newHead)
	if err != nil {
		return block.TipSet{}, err
	}

	newListeners := make([]*chainsampler.HeightThresholdListener, len(m.heightListeners))
	for _, listener := range m.heightListeners {
		valid, err := listener.Handle(ctx, newTips, m.chainState.SampleRandomness)
		if err != nil {
			log.Error("Error checking storage miner chainStore listener", err)
		}

		if valid {
			newListeners = append(newListeners, listener)
		}
	}
	m.heightListeners = newListeners

	return newHead, nil
}

// SendSelfDeals creates self-deals and sends them to the network.
func (m *StorageMinerNodeConnector) SendSelfDeals(ctx context.Context, pieces ...storage.PieceInfo) (cid.Cid, error) {
	proposals := make([]market.ClientDealProposal, len(pieces))
	for i, piece := range pieces {
		prop := market.DealProposal{
			PieceCID:             commcid.PieceCommitmentV1ToCID(piece.CommP[:]),
			PieceSize:            abi.PaddedPieceSize(piece.Size),
			Client:               m.workerAddr,
			Provider:             m.minerAddr,
			StartEpoch:           0,             // TODO populate when the miner module provides this value
			EndEpoch:             math.MaxInt32, // TODO populate when the miner module provides this value
			StoragePricePerEpoch: big.Zero(),
			ProviderCollateral:   big.Zero(),
			ClientCollateral:     big.Zero(),
		}

		proposalBytes, err := encoding.Encode(&proposals[i])
		if err != nil {
			return cid.Undef, err
		}

		sig, err := m.wallet.SignBytes(proposalBytes, m.workerAddr)
		if err != nil {
			return cid.Undef, err
		}

		proposals[i] = market.ClientDealProposal{
			Proposal: prop,
			ClientSignature: crypto.Signature{
				// We know it's BLS from an unrelated requirement.
				// This will go away when we use the new signature type in wallet
				Type: crypto.SigTypeBLS,
				Data: sig,
			},
		}
	}

	params := market.PublishStorageDealsParams{Deals: proposals}
	mcid, cerr, err := m.outbox.Send(
		ctx,
		m.workerAddr,
		vmaddr.StorageMarketAddress,
		types.ZeroAttoFIL,
		types.NewGasPrice(1),
		types.NewGasUnits(300),
		true,
		types.MethodID(builtin.MethodsMarket.PublishStorageDeals),
		&params,
	)
	if err != nil {
		return cid.Undef, err
	}

	err = <-cerr
	if err != nil {
		return cid.Undef, err
	}

	return mcid, nil
}

// WaitForSelfDeals blocks until the provided storage deal-publishing message is
// mined into a block, producing a slice of deal IDs and an exit code when it is
// mined into a block (or an error, if encountered).
func (m *StorageMinerNodeConnector) WaitForSelfDeals(ctx context.Context, mcid cid.Cid) ([]uint64, uint8, error) {
	receiptChan := make(chan *vm.MessageReceipt)
	errChan := make(chan error)

	go func() {
		err := m.waiter.Wait(ctx, mcid, func(b *block.Block, message *types.SignedMessage, r *vm.MessageReceipt) error {
			receiptChan <- r
			return nil
		})
		if err != nil {
			errChan <- err
		}
	}()

	select {
	case receipt := <-receiptChan:
		if receipt.ExitCode != 0 {
			return nil, (uint8)(receipt.ExitCode), nil
		}

		var ret market.PublishStorageDealsReturn
		err := encoding.Decode(receipt.ReturnValue, &ret)
		if err != nil {
			return nil, 0, err
		}

		dealIds := make([]uint64, len(ret.IDs))
		for i, id := range ret.IDs {
			dealIds[i] = uint64(id)
		}
		return dealIds, 0, nil
	case err := <-errChan:
		return nil, 0, err
	case <-ctx.Done():
		return nil, 0, errors.New("context ended prematurely")
	}
}

// SendPreCommitSector creates a pre-commit sector message and sends it to the
// network.
func (m *StorageMinerNodeConnector) SendPreCommitSector(ctx context.Context, sectorID uint64, commR []byte, ticket storage.SealTicket, pieces ...storage.Piece) (cid.Cid, error) {
	dealIds := make([]abi.DealID, len(pieces))
	for i, piece := range pieces {
		dealIds[i] = abi.DealID(piece.DealID)
	}

	params := miner.SectorPreCommitInfo{
		SectorNumber: abi.SectorNumber(sectorID),
		SealedCID:    commcid.ReplicaCommitmentV1ToCID(commR),
		SealEpoch:    abi.ChainEpoch(ticket.BlockHeight),
		DealIDs:      dealIds,
		Expiration:   abi.ChainEpoch(0), // TODO populate when the miner module provides this value.
	}

	mcid, cerr, err := m.outbox.Send(
		ctx,
		m.workerAddr,
		m.minerAddr,
		types.ZeroAttoFIL,
		types.NewGasPrice(1),
		types.NewGasUnits(300),
		true,
		types.MethodID(builtin.MethodsMiner.PreCommitSector),
		&params,
	)
	if err != nil {
		return cid.Undef, err
	}

	err = <-cerr
	if err != nil {
		return cid.Undef, err
	}

	return mcid, nil
}

// WaitForPreCommitSector blocks until the pre-commit sector message is mined
// into a block, returning the block's height and message's exit code (or an
// error if one is encountered).
func (m *StorageMinerNodeConnector) WaitForPreCommitSector(ctx context.Context, mcid cid.Cid) (uint64, uint8, error) {
	return m.waitForMessageHeight(ctx, mcid)
}

// SendProveCommitSector creates a commit sector message and sends it to the
// network.
func (m *StorageMinerNodeConnector) SendProveCommitSector(ctx context.Context, sectorID uint64, proof []byte, deals ...uint64) (cid.Cid, error) {
	dealIds := make([]types.Uint64, len(deals))
	for i, deal := range deals {
		dealIds[i] = types.Uint64(deal)
	}

	params := miner.ProveCommitSectorParams{
		SectorNumber: abi.SectorNumber(sectorID),
		Proof:        abi.SealProof{ProofBytes: proof},
	}

	mcid, cerr, err := m.outbox.Send(
		ctx,
		m.workerAddr,
		m.minerAddr,
		types.ZeroAttoFIL,
		types.NewGasPrice(1),
		types.NewGasUnits(300),
		true,
		types.MethodID(builtin.MethodsMiner.ProveCommitSector),
		&params,
	)
	if err != nil {
		return cid.Undef, err
	}

	err = <-cerr
	if err != nil {
		return cid.Undef, err
	}

	return mcid, nil
}

// WaitForProveCommitSector blocks until the provided pre-commit message has
// been mined into the chainStore, producing the height of the block in which the
// message was mined (and the message's exit code) or an error if any is
// encountered.
func (m *StorageMinerNodeConnector) WaitForProveCommitSector(ctx context.Context, mcid cid.Cid) (uint64, uint8, error) {
	return m.waitForMessageHeight(ctx, mcid)
}

// GetSealTicket produces the seal ticket used when pre-committing a sector at
// the moment it is called
func (m *StorageMinerNodeConnector) GetSealTicket(ctx context.Context) (storage.SealTicket, error) {
	ts, err := m.chainStore.GetTipSet(m.chainStore.GetHead())
	if err != nil {
		return storage.SealTicket{}, xerrors.Errorf("getting head ts for SealTicket failed: %w", err)
	}

	h, err := ts.Height()
	if err != nil {
		return storage.SealTicket{}, err
	}

	r, err := m.chainState.SampleRandomness(ctx, types.NewBlockHeight(h-consensus.FinalityEpochs))
	if err != nil {
		return storage.SealTicket{}, xerrors.Errorf("getting randomness for SealTicket failed: %w", err)
	}

	return storage.SealTicket{
		BlockHeight: h,
		TicketBytes: r,
	}, nil
}

// GetSealSeed is used to acquire the seal seed for the provided pre-commit
// message, and provides channels to accommodate chainStore re-orgs. The caller is
// responsible for choosing an interval-value, which is a quantity of blocks to
// wait (after the block in which the pre-commit message is mined) before
// computing and sampling a seed.
func (m *StorageMinerNodeConnector) GetSealSeed(ctx context.Context, preCommitMsg cid.Cid, interval uint64) (seed <-chan storage.SealSeed, err <-chan error, invalidated <-chan struct{}, done <-chan struct{}) {
	sc := make(chan storage.SealSeed)
	ec := make(chan error)
	ic := make(chan struct{})
	dc := make(chan struct{})

	go func() {
		h, exitCode, err := m.waitForMessageHeight(ctx, preCommitMsg)
		if err != nil {
			ec <- err
			return
		}

		if exitCode != 0 {
			ec <- xerrors.Errorf("non-zero exit code for pre-commit message %d", exitCode)
			return
		}

		m.newListener <- chainsampler.NewHeightThresholdListener(h+interval, sc, ec, ic, dc)
	}()

	return sc, ec, ic, dc
}

type heightAndExitCode struct {
	exitCode uint8
	height   uint64
}

func (m *StorageMinerNodeConnector) waitForMessageHeight(ctx context.Context, mcid cid.Cid) (uint64, uint8, error) {
	height := make(chan heightAndExitCode)
	errChan := make(chan error)

	go func() {
		err := m.waiter.Wait(ctx, mcid, func(b *block.Block, message *types.SignedMessage, r *vm.MessageReceipt) error {
			height <- heightAndExitCode{
				height:   b.Height,
				exitCode: (uint8)(r.ExitCode),
			}
			return nil
		})
		if err != nil {
			errChan <- err
		}
	}()

	select {
	case h := <-height:
		return h.height, h.exitCode, nil
	case err := <-errChan:
		return 0, 0, err
	case <-ctx.Done():
		return 0, 0, errors.New("context ended prematurely")
	}
}
