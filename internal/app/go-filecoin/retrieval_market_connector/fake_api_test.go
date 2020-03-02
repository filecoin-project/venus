package retrievalmarketconnector_test

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/ipfs/go-cid"
	xerrors "github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	retrievalmarketconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/retrieval_market_connector"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// RetrievalMarketClientFakeAPI is a test API that satisfies all needed interface methods
// for a RetrievalMarketClient
type RetrievalMarketClientFakeAPI struct {
	t               *testing.T
	AllocateLaneErr error

	// mock payer's payment channel store by paychAddr
	ExpectedPmtChans map[address.Address]*paymentchannel.ChannelInfo
	// to check that the payment channels were created.
	ActualPmtChans map[address.Address]bool

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
	SaveVoucherErr    error
	ExpectedVouchers  map[address.Address]*paymentchannel.VoucherInfo
	ActualVouchers    map[address.Address]bool

	ExpectedSectorIDs map[uint64]string
	ActualSectorIDs   map[uint64]bool
	UnsealErr         error
}

// NewRetrievalMarketClientFakeAPI creates an instance of a test API that satisfies all needed
// interface methods for a RetrievalMarketClient.
func NewRetrievalMarketClientFakeAPI(t *testing.T, bal abi.TokenAmount) *RetrievalMarketClientFakeAPI {
	return &RetrievalMarketClientFakeAPI{
		t:                 t,
		Balance:           bal,
		WorkerAddr:        requireMakeTestFcAddr(t),
		Nonce:             rand.Uint64(),
		ExpectedPmtChans:  make(map[address.Address]*paymentchannel.ChannelInfo),
		ActualPmtChans:    make(map[address.Address]bool),
		ExpectedVouchers:  make(map[address.Address]*paymentchannel.VoucherInfo),
		ActualVouchers:    make(map[address.Address]bool),
		ExpectedSectorIDs: make(map[uint64]string),
		ActualSectorIDs:   make(map[uint64]bool),
	}
}

// -------------- API METHODS
// AllocateLane mocks allocation of a new lane in a payment channel
func (rmFake *RetrievalMarketClientFakeAPI) AllocateLane(paychAddr address.Address) (uint64, error) {
	_, ok := rmFake.ActualPmtChans[paychAddr]
	if !ok {
		return 0, xerrors.Errorf("payment channel does not exist: %s", paychAddr.String())
	}
	chinfo := rmFake.ExpectedPmtChans[paychAddr]
	states := chinfo.State.LaneStates
	numLanes := len(states)
	ln := paych.LaneState{
		ID:       uint64(numLanes),
		Redeemed: big.NewInt(0),
		Nonce:    1,
	}
	chinfo.State.LaneStates = append(chinfo.State.LaneStates, &ln)
	return ln.ID, rmFake.AllocateLaneErr
}

func (rmFake *RetrievalMarketClientFakeAPI) CreatePaymentChannel(clientAddress, minerAddress address.Address) error {
	if rmFake.CreatePaymentChannelErr != nil {
		return rmFake.CreatePaymentChannelErr
	}
	for paychAddr, chinfo := range rmFake.ExpectedPmtChans {
		if chinfo.State.From == clientAddress && chinfo.State.To == minerAddress {
			rmFake.ActualPmtChans[paychAddr] = true
			return rmFake.CreatePaymentChannelErr
		}
	}
	rmFake.t.Fatalf("unexpected failure in CreatePaymentChannel")
	return nil
}

func (rmFake *RetrievalMarketClientFakeAPI) CreateVoucher(paychAddr address.Address, voucher *paych.SignedVoucher) error {
	return nil
}

// GetPaymentChannelByAccounts mocks returning the channel info for the payment channel
// associated with the account.
// It does not necessarily expect to find the channel info; it returns nil if not found
func (rmFake *RetrievalMarketClientFakeAPI) GetPaymentChannelByAccounts(payer, payee address.Address) (address.Address, *paymentchannel.ChannelInfo) {
	for paychAddr, chinfo := range rmFake.ExpectedPmtChans {
		if chinfo.State.From == payer && chinfo.State.To == payee {
			_, ok := rmFake.ActualPmtChans[paychAddr]
			if ok {
				return paychAddr, chinfo
			}
		}
	}
	return address.Undef, nil
}

// GetChannelInfo mocks getting payment channel info for a payment channel assumed to exist.
// if not found, returns error
func (rmFake *RetrievalMarketClientFakeAPI) GetPaymentChannelInfo(paychAddr address.Address) (*paymentchannel.ChannelInfo, error) {
	// look only at those that have been "created" by moving from Expected to Actual
	_, ok := rmFake.ActualPmtChans[paychAddr]
	if !ok {
		return nil, fmt.Errorf("no such ChannelID %s", paychAddr.String())
	}
	return rmFake.ExpectedPmtChans[paychAddr], nil
}

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
func (rmFake *RetrievalMarketClientFakeAPI) SignBytes(_ []byte, _ address.Address) (crypto.Signature, error) {
	return rmFake.Sig, rmFake.SigErr
}

// Send mocks sending a message on chain
func (rmFake *RetrievalMarketClientFakeAPI) Send(ctx context.Context,
	from, to address.Address,
	value types.AttoFIL,
	gasPrice types.AttoFIL, gasLimit types.GasUnits,
	bcast bool,
	method abi.MethodNum,
	params interface{}) (out cid.Cid, pubErrCh chan error, err error) {
	rmFake.Nonce++

	if err != nil {
		return cid.Undef, nil, err
	}
	return rmFake.MsgSendCid, nil, rmFake.MsgSendErr
}

// SaveVoucher mocks saving a voucher to the payment channel store.
func (rmFake *RetrievalMarketClientFakeAPI) SaveVoucher(paychAddr address.Address, voucher *paych.SignedVoucher, proof []byte, expectedAmt abi.TokenAmount) (abi.TokenAmount, error) {
	_, ok := rmFake.ExpectedVouchers[paychAddr]
	if !ok {
		rmFake.t.Fatalf("missing voucher for %s", paychAddr.String())
	}
	if rmFake.SaveVoucherErr != nil {
		return abi.NewTokenAmount(0), rmFake.SaveVoucherErr
	}
	rmFake.ActualVouchers[paychAddr] = true
	return expectedAmt, nil
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
	assert.Equal(rmFake.t, len(rmFake.ExpectedPmtChans), len(rmFake.ActualPmtChans))
	assert.Equal(rmFake.t, len(rmFake.ActualVouchers), len(rmFake.ExpectedVouchers))
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

var _ retrievalmarketconnector.PaychMgrAPI = &RetrievalMarketClientFakeAPI{}
var _ retrievalmarketconnector.RetrievalSigner = &RetrievalMarketClientFakeAPI{}
var _ retrievalmarketconnector.UnsealerAPI = &RetrievalMarketClientFakeAPI{}
