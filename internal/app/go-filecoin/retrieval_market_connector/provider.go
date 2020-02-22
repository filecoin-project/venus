package retrievalmarketconnector

import (
	"context"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
)

// RetrievalProviderConnector is the glue between go-filecoin and retrieval market provider API
type RetrievalProviderConnector struct {
	paychMgr PaychMgrAPI
	ps       piecestore.PieceStore
	bs       blockstore.Blockstore
	net      rmnet.RetrievalMarketNetwork
}

var _ retrievalmarket.RetrievalProviderNode = &RetrievalProviderConnector{}

// NewRetrievalProviderConnector creates a new RetrievalProviderConnector
func NewRetrievalProviderConnector(network rmnet.RetrievalMarketNetwork, pieceStore piecestore.PieceStore, bs blockstore.Blockstore, paychMgr PaychMgrAPI) *RetrievalProviderConnector {
	return &RetrievalProviderConnector{
		ps:       pieceStore,
		bs:       bs,
		net:      network,
		paychMgr: paychMgr,
	}
}

// UnsealSector unseals the sector given by sectorId and offset with length `length`
func (r *RetrievalProviderConnector) UnsealSector(ctx context.Context, sectorId uint64, offset uint64, length uint64) (io.ReadCloser, error) {
	panic("implement UnsealSector")
	return nil, nil
}

// SavePaymentVoucher stores the provided payment voucher with the payment channel actor
func (r *RetrievalProviderConnector) SavePaymentVoucher(_ context.Context, paymentChannel address.Address, voucher *paych.SignedVoucher, proof []byte, expected abi.TokenAmount) (abi.TokenAmount, error) {

	_, err := r.paychMgr.GetPaymentChannelInfo(paymentChannel)
	if err != nil {
		return abi.NewTokenAmount(0), err
	}
	// provider attempts to redeem voucher
	// (totalSent * pricePerbyte) - fundsReceived
	// on return the retrievalMarket asks the client for more fund if recorded available
	// amount in channel is less than expectedAmt
	// how much validation here?

	actual, err := r.paychMgr.SaveVoucher(paymentChannel, voucher, proof, expected)
	if err != nil {
		return abi.NewTokenAmount(0), err
	}

	return actual, nil
}
