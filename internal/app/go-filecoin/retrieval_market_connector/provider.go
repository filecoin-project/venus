package retrievalmarketconnector

import (
	"context"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	rtypes "github.com/filecoin-project/go-fil-markets/shared/types"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	xerrors "github.com/pkg/errors"
)

type RetrievalProviderConnector struct {
	vs  map[string]voucherEntry
	ps  piecestore.PieceStore
	bs  blockstore.Blockstore
	net rmnet.RetrievalMarketNetwork
}

var _ retrievalmarket.RetrievalProviderNode = &RetrievalProviderConnector{}

// voucherEntry keeps track of how much has been paid
type voucherEntry struct {
	voucher     *rtypes.SignedVoucher
	proof       []byte
	expectedAmt tokenamount.TokenAmount
}


func NewRetrievalProviderNodeConnector(network rmnet.RetrievalMarketNetwork, pieceStore piecestore.PieceStore, bs *blockstore.Blockstore) *RetrievalProviderConnector {
	return &RetrievalProviderConnector{
		vs:  make(map[string]voucherEntry),
		ps:  pieceStore,
		bs:  bs,
		net: network,
	}
}

func (r *RetrievalProviderNodeConnector) UnsealSector(ctx context.Context, sectorId uint64, offset uint64, length uint64) (io.ReadCloser, error) {
	panic("TODO: go-fil-markets integration")
}

func (r *RetrievalProviderNodeConnector) SavePaymentVoucher(_ context.Context, paymentChannel address.Address, voucher *paych.SignedVoucher, proof []byte, expectedAmount abi.TokenAmount) (abi.TokenAmount, error) {
	var tokenamt tokenamount.TokenAmount

	key, err := r.voucherStoreKeyFor(voucher)
	if err != nil {
		return tokenamt, err
	}
	_, ok := r.vs[key]
	if ok {
		return tokenamt, xerrors.New("voucher exists")
	}
	r.vs[key] = voucherEntry{
		voucher:     voucher,
		proof:       proof,
		expectedAmt: expectedAmount,
	}
	return voucher.Amount, nil
}

func (r *RetrievalProviderConnector) voucherStoreKeyFor(voucher *rtypes.SignedVoucher) (string, error) {
	venc, err := voucher.EncodedString()
	if err != nil {
		return "", err
	}
	return venc, nil
}
