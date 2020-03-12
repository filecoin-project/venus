package retrievalmarketconnector

import (
	"context"
	"io"
	"math/rand"
	"os"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/ipfs/go-cid"
	xerrors "github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// RetrievalMarketClientFakeAPI is a test API that satisfies all needed interface methods
// for a RetrievalMarketClient
type RetrievalMarketClientFakeAPI struct {
	t               *testing.T
	AllocateLaneErr error

	PayChBalanceErr error

	Balance                 abi.TokenAmount
	BalanceErr              error
	CreatePaymentChannelErr error
	WorkerAddr              address.Address
	WorkerAddrErr           error
	Nonce                   uint64
	NonceErr                error

	Sig    crypto.Signature
	SigErr error

	MsgSendCid cid.Cid
	MsgSendErr error

	SendNewVoucherErr error
	ExpectedVouchers  map[address.Address]*paymentchannel.VoucherInfo
	ActualVouchers    map[address.Address]bool

	ExpectedSectorIDs map[uint64]string
	ActualSectorIDs   map[uint64]bool
	UnsealErr         error
}

func (rmFake *RetrievalMarketClientFakeAPI) ChannelExists(_ address.Address) (bool, error) {
	return true, nil
}

// NewRetrievalMarketClientFakeAPI creates an instance of a test API that satisfies all needed
// interface methods for a RetrievalMarketClient.
func NewRetrievalMarketClientFakeAPI(t *testing.T, bal abi.TokenAmount) *RetrievalMarketClientFakeAPI {
	return &RetrievalMarketClientFakeAPI{
		t:                 t,
		Balance:           bal,
		WorkerAddr:        requireMakeTestFcAddr(t),
		Nonce:             rand.Uint64(),
		ExpectedVouchers:  make(map[address.Address]*paymentchannel.VoucherInfo),
		ActualVouchers:    make(map[address.Address]bool),
		ExpectedSectorIDs: make(map[uint64]string),
		ActualSectorIDs:   make(map[uint64]bool),
	}
}

// -------------- API METHODS
// GetBalance mocks getting an actor's balance in AttoFIL
func (rmFake *RetrievalMarketClientFakeAPI) GetBalance(_ context.Context, _ address.Address) (types.AttoFIL, error) {
	return types.NewAttoFIL(rmFake.Balance.Int), rmFake.BalanceErr
}

// NextNonce mocks getting an actor's next nonce
func (rmFake *RetrievalMarketClientFakeAPI) NextNonce(_ context.Context, _ address.Address) (uint64, error) {
	rmFake.Nonce++
	return rmFake.Nonce, rmFake.NonceErr
}

// SignBytes mocks signing data
func (rmFake *RetrievalMarketClientFakeAPI) SignBytes(data []byte, addr address.Address) (crypto.Signature, error) {
	return rmFake.Sig, rmFake.SigErr
}

// UnsealSector mocks unsealing.  Assign a filename to ExpectedSectorIDs[sectorID] to
// test
func (rmFake *RetrievalMarketClientFakeAPI) UnsealSector(_ context.Context, sectorID uint64) (io.ReadCloser, error) {
	if rmFake.UnsealErr != nil {
		return nil, rmFake.UnsealErr
	}
	name, ok := rmFake.ExpectedSectorIDs[sectorID]
	if !ok {
		return nil, xerrors.New("RetrievalMarketClientFakeAPI: sectorID does not exist")
	}
	rc, err := os.OpenFile(name, os.O_RDONLY, 0500)
	require.NoError(rmFake.t, err)
	rmFake.ActualSectorIDs[sectorID] = true
	return rc, nil
}

// ---------------  Testing methods
func (rmFake *RetrievalMarketClientFakeAPI) Verify() {
	assert.Equal(rmFake.t, len(rmFake.ActualSectorIDs), len(rmFake.ExpectedSectorIDs))
}

// StubMessageResponse sets up a message, message receipt and return value for a create payment
// channel message
func (rmFake *RetrievalMarketClientFakeAPI) StubSignature(sigError error) {
	mockSigner, _ := types.NewMockSignersAndKeyInfo(1)
	addr1 := mockSigner.Addresses[0]

	sig, err := mockSigner.SignBytes([]byte("pork chops and applesauce"), addr1)
	require.NoError(rmFake.t, err)

	signature := crypto.Signature{
		Type: crypto.SigTypeBLS,
		Data: sig.Data,
	}
	rmFake.Sig = signature
	rmFake.SigErr = sigError
}

// requireMakeTestFcAddr generates a random ID addr for test
func requireMakeTestFcAddr(t *testing.T) address.Address {
	res, err := address.NewIDAddress(rand.Uint64())
	require.NoError(t, err)
	return res
}

var _ RetrievalSigner = &RetrievalMarketClientFakeAPI{}
var _ UnsealerAPI = &RetrievalMarketClientFakeAPI{}
