package retrievalmarketconnector

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	"github.com/filecoin-project/go-fil-markets/shared/types"
)

// RetrievalClientNodeConnector adapts the node to provide an interface used by the retrieval client.
type RetrievalClientNodeConnector struct{}

// NewRetrievalClientNodeConnector creates a new connector.
func NewRetrievalClientNodeConnector() *RetrievalClientNodeConnector {
	return &RetrievalClientNodeConnector{}
}

// GetOrCreatePaymentChannel retrieves a payment channel for the retrieval client.
func (r *RetrievalClientNodeConnector) GetOrCreatePaymentChannel(ctx context.Context, clientAddress address.Address, minerAddress address.Address, clientFundsAvailable tokenamount.TokenAmount) (address.Address, error) {
	panic("TODO: go-fil-markets integration")
}

// AllocateLane creates a lane for the retrieval client.
func (r *RetrievalClientNodeConnector) AllocateLane(paymentChannel address.Address) (uint64, error) {
	panic("TODO: go-fil-markets integration")
}

// CreatePaymentVoucher creates a payment voucher for the retrieval client.
func (r *RetrievalClientNodeConnector) CreatePaymentVoucher(ctx context.Context, paymentChannel address.Address, amount tokenamount.TokenAmount, lane uint64) (*types.SignedVoucher, error) {
	panic("TODO: go-fil-markets integration")
}
