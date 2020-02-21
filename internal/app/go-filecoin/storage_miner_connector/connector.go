package storageminerconnector

import (
	"context"
	"errors"

	"github.com/filecoin-project/go-address"
	storagenode "github.com/filecoin-project/go-storage-miner/apis/node"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsampler"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
)

type randomnessSampler interface {
	SampleRandomness(ctx context.Context, sampleHeight abi.ChainEpoch) ([]byte, error)
	GetTipSetStateRoot(ctx context.Context, tipKey block.TipSetKey) (cid.Cid, error)
	GetTipSet(key block.TipSetKey) (block.TipSet, error)
	Head() block.TipSetKey
}

// StorageMinerNodeConnector is a struct which satisfies the go-storage-miner
// needs of "the node," e.g. interacting with the blockchain, persisting sector
// states to disk, and so forth.
type StorageMinerNodeConnector struct {
	minerAddr address.Address

	chainHeightScheduler *chainsampler.HeightThresholdScheduler

	chainStore *chain.Store
	chainState randomnessSampler
	outbox     *message.Outbox
	waiter     *msg.Waiter
	wallet     *wallet.Wallet

	stateViewer *appstate.Viewer
}

var _ storagenode.Interface = new(StorageMinerNodeConnector)

// NewStorageMinerNodeConnector produces a StorageMinerNodeConnector, which adapts
// types in this codebase to the interface representing "the node" which is
// expected by the go-storage-miner project.
func NewStorageMinerNodeConnector(minerAddress address.Address, chainStore *chain.Store, chainState randomnessSampler, outbox *message.Outbox, waiter *msg.Waiter, wallet *wallet.Wallet, stateViewer *appstate.Viewer) *StorageMinerNodeConnector {
	return &StorageMinerNodeConnector{
		minerAddr:            minerAddress,
		chainHeightScheduler: chainsampler.NewHeightThresholdScheduler(chainStore),
		chainStore:           chainStore,
		chainState:           chainState,
		outbox:               outbox,
		waiter:               waiter,
		wallet:               wallet,
		stateViewer:          stateViewer,
	}
}

// StartHeightListener starts the scheduler that manages height listeners.
func (m *StorageMinerNodeConnector) StartHeightListener(ctx context.Context, htc <-chan interface{}) {
	m.chainHeightScheduler.StartHeightListener(ctx, htc)
}

// StopHeightListener stops the scheduler that manages height listeners.
func (m *StorageMinerNodeConnector) StopHeightListener() {
	m.chainHeightScheduler.Stop()
}

func (m *StorageMinerNodeConnector) handleNewTipSet(ctx context.Context, previousHead block.TipSet) (block.TipSet, error) {

	return m.chainHeightScheduler.HandleNewTipSet(ctx, previousHead)
}

// SendSelfDeals creates self-deals and sends them to the network.
func (m *StorageMinerNodeConnector) SendSelfDeals(ctx context.Context, startEpoch, endEpoch abi.ChainEpoch, pieces ...abi.PieceInfo) (cid.Cid, error) {
	waddr, err := m.getMinerWorkerAddress(ctx, m.chainState.Head())
	if err != nil {
		return cid.Undef, err
	}

	proposals := make([]market.ClientDealProposal, len(pieces))
	for i, piece := range pieces {
		proposals[i] = market.ClientDealProposal{
			Proposal: market.DealProposal{
				PieceCID:             piece.PieceCID,
				PieceSize:            piece.Size,
				Client:               waddr,
				Provider:             m.minerAddr,
				StartEpoch:           startEpoch,
				EndEpoch:             endEpoch,
				StoragePricePerEpoch: abi.NewTokenAmount(0),
				ProviderCollateral:   abi.NewTokenAmount(0),
				ClientCollateral:     abi.NewTokenAmount(0),
			},
		}

		buf, err := encoding.Encode(proposals[i])
		if err != nil {
			return cid.Undef, err
		}

		sig, err := m.wallet.SignBytes(buf, waddr)
		if err != nil {
			return cid.Undef, err
		}

		proposals[i].ClientSignature = sig
	}

	params := market.PublishStorageDealsParams{Deals: proposals}

	mcid, cerr, err := m.outbox.Send(
		ctx,
		waddr,
		builtin.StorageMarketActorAddr,
		types.ZeroAttoFIL,
		types.NewGasPrice(1),
		types.GasUnits(300),
		true,
		builtin.MethodsMarket.PublishStorageDeals,
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
func (m *StorageMinerNodeConnector) WaitForSelfDeals(ctx context.Context, mcid cid.Cid) ([]abi.DealID, uint8, error) {
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
		dealIdsPrime := make([]abi.DealID, len(dealIds))
		for idx := range dealIds {
			dealIdsPrime[idx] = abi.DealID(dealIds[idx])
		}

		return dealIdsPrime, 0, nil
	case err := <-errChan:
		return nil, 0, err
	case <-ctx.Done():
		return nil, 0, errors.New("context ended prematurely")
	}
}

// SendPreCommitSector creates a pre-commit sector message and sends it to the
// network.
func (m *StorageMinerNodeConnector) SendPreCommitSector(ctx context.Context, sectorNum abi.SectorNumber, sealedCID cid.Cid, sealEpoch, expiration abi.ChainEpoch, pieces ...storagenode.PieceWithDealInfo) (cid.Cid, error) {
	waddr, err := m.getMinerWorkerAddress(ctx, m.chainState.Head())
	if err != nil {
		return cid.Undef, err
	}

	dealIds := make([]abi.DealID, len(pieces))
	for i, piece := range pieces {
		dealIds[i] = piece.DealInfo.DealID
	}

	params := miner.SectorPreCommitInfo{
		SectorNumber: sectorNum,
		SealedCID:    sealedCID,
		SealEpoch:    sealEpoch,
		DealIDs:      dealIds,
		Expiration:   expiration,
	}

	mcid, cerr, err := m.outbox.Send(
		ctx,
		waddr,
		m.minerAddr,
		types.ZeroAttoFIL,
		types.NewGasPrice(1),
		types.GasUnits(300),
		true,
		builtin.MethodsMiner.PreCommitSector,
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
func (m *StorageMinerNodeConnector) WaitForPreCommitSector(ctx context.Context, mcid cid.Cid) (abi.ChainEpoch, uint8, error) {
	return m.waitForMessageHeight(ctx, mcid)
}

// SendProveCommitSector creates a commit sector message and sends it to the
// network.
func (m *StorageMinerNodeConnector) SendProveCommitSector(ctx context.Context, sectorNum abi.SectorNumber, proof []byte, deals ...abi.DealID) (cid.Cid, error) {
	waddr, err := m.getMinerWorkerAddress(ctx, m.chainState.Head())
	if err != nil {
		return cid.Undef, err
	}

	params := miner.ProveCommitSectorParams{
		SectorNumber: sectorNum,
		Proof:        abi.SealProof{ProofBytes: proof},
	}

	mcid, cerr, err := m.outbox.Send(
		ctx,
		waddr,
		m.minerAddr,
		types.ZeroAttoFIL,
		types.NewGasPrice(1),
		types.GasUnits(300),
		true,
		builtin.MethodsMiner.ProveCommitSector,
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
func (m *StorageMinerNodeConnector) WaitForProveCommitSector(ctx context.Context, mcid cid.Cid) (uint8, error) {
	_, exitCode, err := m.waitForMessageHeight(ctx, mcid)
	return exitCode, err
}

// GetSealTicket produces the seal ticket used when pre-committing a sector.
func (m *StorageMinerNodeConnector) GetSealTicket(ctx context.Context, tok storagenode.TipSetToken) (storagenode.SealTicket, error) {
	var tsk block.TipSetKey
	if err := tsk.UnmarshalCBOR(tok); err != nil {
		return storagenode.SealTicket{}, xerrors.Errorf("failed to marshal TipSetToken into a TipSetKey: %w", err)
	}

	ts, err := m.chainStore.GetTipSet(tsk)
	if err != nil {
		return storagenode.SealTicket{}, xerrors.Errorf("getting head ts for SealTicket failed: %w", err)
	}

	h, err := ts.Height()
	if err != nil {
		return storagenode.SealTicket{}, err
	}

	r, err := m.chainState.SampleRandomness(ctx, h-consensus.FinalityEpochs)
	if err != nil {
		return storagenode.SealTicket{}, xerrors.Errorf("getting randomness for SealTicket failed: %w", err)
	}

	return storagenode.SealTicket{
		BlockHeight: uint64(h),
		TicketBytes: r,
	}, nil
}

func (m *StorageMinerNodeConnector) GetChainHead(ctx context.Context) (storagenode.TipSetToken, abi.ChainEpoch, error) {
	tsk := m.chainState.Head()

	ts, err := m.chainState.GetTipSet(tsk)
	if err != nil {
		return nil, 0, xerrors.Errorf("failed to get tip: %w", err)
	}

	h, err := ts.Height()
	if err != nil {
		return nil, 0, xerrors.Errorf("failed to get tipset height: %w")
	}

	tok, err := encoding.Encode(tsk)
	if err != nil {
		return nil, 0, xerrors.Errorf("failed to marshal TipSetKey to CBOR byte slice for TipSetToken: %w", err)
	}

	return tok, h, nil
}

// GetSealSeed is used to acquire the seal seed for the provided pre-commit
// message, and provides channels to accommodate chainStore re-orgs. The caller is
// responsible for choosing an interval-value, which is a quantity of blocks to
// wait (after the block in which the pre-commit message is mined) before
// computing and sampling a seed.
func (m *StorageMinerNodeConnector) GetSealSeed(ctx context.Context, preCommitMsg cid.Cid, interval uint64) (<-chan storagenode.SealSeed, <-chan storagenode.SeedInvalidated, <-chan storagenode.FinalityReached, <-chan storagenode.GetSealSeedError) {
	sc := make(chan storagenode.SealSeed)
	ec := make(chan storagenode.GetSealSeedError)
	ic := make(chan storagenode.SeedInvalidated)
	dc := make(chan storagenode.FinalityReached)

	go func() {
		h, exitCode, err := m.waitForMessageHeight(ctx, preCommitMsg)
		if err != nil {
			ec <- storagenode.NewGetSealSeedError(err, storagenode.GetSealSeedFailedError)
			return
		}

		if exitCode != 0 {
			err := xerrors.Errorf("non-zero exit code for pre-commit message %d", exitCode)
			ec <- storagenode.NewGetSealSeedError(err, storagenode.GetSealSeedFailedError)
			return
		}

		listener := m.chainHeightScheduler.AddListener(h + abi.ChainEpoch(interval))

		// translate tipset key to seal seed handler
		for {
			select {
			case key := <-listener.HitCh:
				ts, err := m.chainState.GetTipSet(key)
				if err != nil {
					ec <- storagenode.NewGetSealSeedError(err, storagenode.GetSealSeedFatalError)
					break
				}

				tsHeight, err := ts.Height()
				if err != nil {
					ec <- storagenode.NewGetSealSeedError(err, storagenode.GetSealSeedFatalError)
					break
				}

				randomness, err := m.chainState.SampleRandomness(ctx, tsHeight)
				if err != nil {
					ec <- storagenode.NewGetSealSeedError(err, storagenode.GetSealSeedFatalError)
					break
				}

				sc <- storagenode.SealSeed{
					BlockHeight: uint64(tsHeight),
					TicketBytes: randomness,
				}
			case err := <-listener.ErrCh:
				ec <- storagenode.NewGetSealSeedError(err, storagenode.GetSealSeedFailedError)
			case <-listener.InvalidCh:
				ic <- storagenode.SeedInvalidated{}
			case <-listener.DoneCh:
				dc <- storagenode.FinalityReached{}
				return
			case <-ctx.Done():
				m.chainHeightScheduler.CancelListener(listener)
				return
			}
		}
	}()

	return sc, ic, dc, ec
}

type heightAndExitCode struct {
	exitCode uint8
	height   abi.ChainEpoch
}

func (m *StorageMinerNodeConnector) waitForMessageHeight(ctx context.Context, mcid cid.Cid) (abi.ChainEpoch, uint8, error) {
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

func (m *StorageMinerNodeConnector) GetMinerWorkerAddress(ctx context.Context, tok storagenode.TipSetToken) (address.Address, error) {
	var tsk block.TipSetKey
	if err := encoding.Decode(tok, &tsk); err != nil {
		return address.Undef, xerrors.Errorf("failed to marshal TipSetToken into a TipSetKey: %w", err)
	}

	return m.getMinerWorkerAddress(ctx, tsk)
}

func (m *StorageMinerNodeConnector) SendReportFaults(ctx context.Context, sectorIDs ...abi.SectorNumber) (cid.Cid, error) {
	waddr, err := m.getMinerWorkerAddress(ctx, m.chainState.Head())
	if err != nil {
		return cid.Undef, err
	}

	bf := abi.NewBitField()
	for _, id := range sectorIDs {
		bf.Set(uint64(id))
	}

	params := miner.DeclareTemporaryFaultsParams{
		SectorNumbers: bf,

		// TODO: use a real value here
		Duration: abi.ChainEpoch(miner.ProvingPeriod),
	}

	mcid, cerr, err := m.outbox.Send(
		ctx,
		waddr,
		m.minerAddr,
		types.ZeroAttoFIL,
		types.NewGasPrice(1),
		types.GasUnits(300),
		true,
		builtin.MethodsMiner.DeclareTemporaryFaults,
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

func (m *StorageMinerNodeConnector) WaitForReportFaults(ctx context.Context, msgCid cid.Cid) (uint8, error) {
	_, exitCode, err := m.waitForMessageHeight(ctx, msgCid)
	return exitCode, err
}

func (m *StorageMinerNodeConnector) GetSealedCID(ctx context.Context, tok storagenode.TipSetToken, sectorNum abi.SectorNumber) (sealedCID cid.Cid, wasFound bool, err error) {
	var tsk block.TipSetKey
	if err := tsk.UnmarshalCBOR(tok); err != nil {
		return cid.Undef, false, xerrors.Errorf("failed to marshal TipSetToken into a TipSetKey: %w", err)
	}

	root, err := m.chainState.GetTipSetStateRoot(ctx, tsk)
	if err != nil {
		return cid.Undef, false, xerrors.Errorf("failed to get tip state: %w", err)
	}

	preCommitInfo, found, err := m.stateViewer.StateView(root).MinerGetPrecommittedSector(ctx, m.minerAddr, uint64(sectorNum))
	if !found || err != nil {
		return cid.Undef, found, err
	}

	return preCommitInfo.Info.SealedCID, true, nil
}

func (m *StorageMinerNodeConnector) CheckPieces(ctx context.Context, sectorNum abi.SectorNumber, pieces []storagenode.PieceWithDealInfo) *storagenode.CheckPiecesError {
	return nil
}

func (m *StorageMinerNodeConnector) CheckSealing(ctx context.Context, commD []byte, dealIDs []abi.DealID, ticket storagenode.SealTicket) *storagenode.CheckSealingError {
	return nil
}

func (m *StorageMinerNodeConnector) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	return m.wallet.HasAddress(addr), nil
}

func (m *StorageMinerNodeConnector) getMinerWorkerAddress(ctx context.Context, tsk block.TipSetKey) (address.Address, error) {
	root, err := m.chainState.GetTipSetStateRoot(ctx, tsk)
	if err != nil {
		return address.Undef, xerrors.Errorf("failed to get tip state: %w", err)
	}

	_, waddr, err := m.stateViewer.StateView(root).MinerControlAddresses(ctx, m.minerAddr)
	if err != nil {
		return address.Undef, xerrors.Errorf("failed to get miner control addresses: %w", err)
	}

	return waddr, nil
}
