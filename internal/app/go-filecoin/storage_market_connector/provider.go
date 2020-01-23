package storagemarketconnector

import (
	"context"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	t2 "github.com/filecoin-project/go-fil-markets/shared/types"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	fcsm "github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/storagemarket"
	fcaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

type StorageProviderNodeConnector struct {
	minerAddr  address.Address
	chainStore *chain.Store
	outbox     *message.Outbox
	waiter     *msg.Waiter
	//wallet     *wallet.Wallet
}

func NewStorageProviderNodeConnector() *StorageProviderNodeConnector {
	return &StorageProviderNodeConnector{}
}

func (s *StorageProviderNodeConnector) MostRecentStateId(ctx context.Context) (storagemarket.StateKey, error) {
	key := s.chainStore.GetHead()
	ts, err := s.chainStore.GetTipSet(key)

	if err != nil {
		return nil, err
	}

	return &stateKey{key, uint64(ts.At(0).Height)}, nil
}

func (s *StorageProviderNodeConnector) AddFunds(ctx context.Context, addr address.Address, amount tokenamount.TokenAmount) error {
	params, err := abi.ToEncodedValues(addr)
	if err != nil {
		return err
	}

	workerAddr, err := s.getFCWorker(ctx)
	if err != nil {
		return err
	}

	mcid, cerr, err := s.outbox.Send(
		ctx,
		workerAddr,
		fcaddr.StorageMarketAddress,
		types.NewAttoFIL(amount.Int),
		types.NewGasPrice(1),
		types.NewGasUnits(300),
		true,
		fcsm.AddBalance,
		params,
	)
	if err != nil {
		return err
	}

	_, err = s.wait(ctx, mcid, cerr)

	return err
}

func (s *StorageProviderNodeConnector) EnsureFunds(ctx context.Context, addr address.Address, amount tokenamount.TokenAmount) error {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) GetBalance(ctx context.Context, addr address.Address) (storagemarket.Balance, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) PublishDeals(ctx context.Context, deal storagemarket.MinerDeal) (storagemarket.DealID, cid.Cid, error) {
	client, err := fcaddr.NewFromBytes(deal.Proposal.Client.Bytes())
	if err != nil {
		return 0, cid.Undef, err
	}

	provider, err := fcaddr.NewFromBytes(deal.Proposal.Provider.Bytes())
	if err != nil {
		return 0, cid.Undef, err
	}

	sig := types.Signature(deal.Proposal.ProposerSignature.Data)

	fcStorageProposal := types.StorageDealProposal{
		PieceRef:  deal.Proposal.PieceRef,
		PieceSize: types.Uint64(deal.Proposal.PieceSize),

		Client:   client,
		Provider: provider,

		ProposalExpiration: types.Uint64(deal.Proposal.ProposalExpiration),
		Duration:           types.Uint64(deal.Proposal.Duration),

		StoragePricePerEpoch: types.Uint64(deal.Proposal.StoragePricePerEpoch.Uint64()),
		StorageCollateral:    types.Uint64(deal.Proposal.StorageCollateral.Uint64()),

		ProposerSignature: &sig,
	}
	params, err := abi.ToEncodedValues([]types.StorageDealProposal{fcStorageProposal})
	if err != nil {
		return 0, cid.Undef, err
	}

	workerAddr, err := s.getFCWorker(ctx)
	if err != nil {
		return err
	}

	mcid, cerr, err := s.outbox.Send(
		ctx,
		workerAddr,
		fcaddr.StorageMarketAddress,
		types.ZeroAttoFIL,
		types.NewGasPrice(1),
		types.NewGasUnits(300),
		true,
		fcsm.PublishStorageDeals,
		params,
	)
	if err != nil {
		return 0, cid.Undef, err
	}

	receipt, err := s.wait(ctx, mcid, cerr)

	dealIDValues, err := abi.Deserialize(receipt.Return[0], abi.UintArray)
	if err != nil {
		return 0, cid.Undef, err
	}

	dealIds, ok := dealIDValues.Val.([]uint64)
	if !ok {
		return 0, cid.Undef, xerrors.New("decoded deal ids are not a []uint64")
	}

	if len(dealIds) < 1 {
		return 0, cid.Undef, xerrors.New("Successful call to publish storage deals did not return deal ids")
	}

	return storagemarket.DealID(dealIds[0]), mcid, err
}

func (s *StorageProviderNodeConnector) ListProviderDeals(ctx context.Context, addr address.Address) ([]storagemarket.StorageDeal, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) OnDealComplete(ctx context.Context, deal storagemarket.MinerDeal, pieceSize uint64, pieceReader io.Reader) (uint64, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) GetMinerWorker(ctx context.Context, miner address.Address) (address.Address, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) SignBytes(ctx context.Context, signer address.Address, b []byte) (*t2.Signature, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) OnDealSectorCommitted(ctx context.Context, provider address.Address, dealID uint64, cb storagemarket.DealSectorCommittedCallback) error {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) LocatePieceForDealWithinSector(ctx context.Context, dealID uint64) (sectorID uint64, offset uint64, length uint64, err error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) wait(ctx context.Context, mcid cid.Cid, pubErrCh chan error) (*types.MessageReceipt, error) {
	receiptChan := make(chan *types.MessageReceipt)
	errChan := make(chan error)

	err := <-pubErrCh
	if err != nil {
		return nil, err
	}

	go func() {
		err := s.waiter.Wait(ctx, mcid, func(b *block.Block, message *types.SignedMessage, r *types.MessageReceipt) error {
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
			return nil, xerrors.Errorf("non-zero exit code: %d", receipt.ExitCode)
		}

		return receipt, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, xerrors.New("context ended prematurely")
	}
}

func (s *StorageProviderNodeConnector) getFCWorker(ctx context.Context) (fcaddr.Address, error) {
	worker, err := s.GetMinerWorker(ctx, s.minerAddr)
	if err != nil {
		return fcaddr.Undef, err
	}

	workerAddr, err := fcaddr.NewFromBytes(worker.Bytes())
	if err != nil {
		return fcaddr.Undef, err
	}
	return workerAddr, nil
}
