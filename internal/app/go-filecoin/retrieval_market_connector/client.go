package retrievalmarketconnector

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	"github.com/filecoin-project/go-fil-markets/shared/types"
)

type RetrievalClientNodeConnector struct{}

func NewRetrievalClientNodeConnector() *RetrievalClientNodeConnector {
	return &RetrievalClientNodeConnector{}
}

func (r *RetrievalClientNodeConnector) GetOrCreatePaymentChannel(ctx context.Context, clientAddress address.Address, minerAddress address.Address, clientFundsAvailable tokenamount.TokenAmount) (address.Address, error) {
	panic("TODO: go-fil-markets integration")
}

func (r *RetrievalClientNodeConnector) AllocateLane(paymentChannel address.Address) (uint64, error) {
	panic("TODO: go-fil-markets integration")
}

func (r *RetrievalClientNodeConnector) CreatePaymentVoucher(ctx context.Context, paymentChannel address.Address, amount tokenamount.TokenAmount, lane uint64) (*types.SignedVoucher, error) {
	panic("TODO: go-fil-markets integration")
}
