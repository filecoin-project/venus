package retrievalmarketconnector

import (
	"context"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	retmkt "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
)

// RetrievalProviderConnector is the glue between go-filecoin and retrieval market provider API
type RetrievalProviderConnector struct {
	bs       blockstore.Blockstore
	ps       piecestore.PieceStore
	net      rmnet.RetrievalMarketNetwork
	paychMgr PaychMgrAPI
	provider retmkt.RetrievalProvider
}

var _ retmkt.RetrievalProviderNode = &RetrievalProviderConnector{}

// NewRetrievalProviderConnector creates a new RetrievalProviderConnector
func NewRetrievalProviderConnector(net rmnet.RetrievalMarketNetwork, ps piecestore.PieceStore,
	bs blockstore.Blockstore, paychMgr PaychMgrAPI) *RetrievalProviderConnector {
	return &RetrievalProviderConnector{
		ps:       ps,
		bs:       bs,
		net:      net,
		paychMgr: paychMgr,
	}
}

// SetProvider sets the retrieval provider for the RetrievalProviderConnector
func (r *RetrievalProviderConnector) SetProvider(provider retmkt.RetrievalProvider) {
	r.provider = provider
}

// UnsealSector unseals the sector given by sectorId and offset with length `length`
func (r *RetrievalProviderConnector) UnsealSector(ctx context.Context, sectorID uint64,
	offset uint64, length uint64) (io.ReadCloser, error) {
	panic("implement UnsealSector")
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

	actual, err := r.paychMgr.SaveVoucher(paymentChannel, voucher, proof, expected)
	if err != nil {
		return abi.NewTokenAmount(0), err
	}

	return actual, nil
}
