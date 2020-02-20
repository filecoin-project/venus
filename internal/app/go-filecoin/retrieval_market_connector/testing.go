package retrievalmarketconnector

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	paychActor "github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

// RetrievalMarketClientFakeAPI is a test API that satisfies all needed interface methods
// for a RetrievalMarketClient
type RetrievalMarketClientFakeAPI struct {
	AllocateLaneErr error

	ExpectedLanes    map[address.Address]int64        // mock payment broker lane store
	ExpectedPmtChans map[address.Address]PmtChanEntry // mock payment broker's payment channel store
	ActualPmtChans   map[address.Address]PmtChanEntry // to check that the payment channels were created

	PayChBalanceErr error

	Balance       abi.TokenAmount
	BalanceErr    error
	WorkerAddr    address.Address
	WorkerAddrErr error
	Nonce         uint64
	NonceErr      error
	Sig           *crypto.Signature
	SigErr        error
	MsgSendCid    cid.Cid
	MsgSendErr    error
	WaitErr       error

	ExpectedBlock      *block.Block
	ExpectedMsgReceipt *vm.MessageReceipt
	ExpectedSignedMsg  *types.SignedMessage
}

// PmtChanEntry is a mock record of a created payment channel with funds available.
// TODO: this will change to reflect it being an actor
type PmtChanEntry struct {
	Payee      address.Address
	Redeemed   abi.TokenAmount
	ChannelID  address.Address
	FundsAvail abi.TokenAmount
}

// NewRetrievalMarketClientFakeAPI creates an instance of a test API that satisfies all needed
// interface methods for a RetrievalMarketClient.
func NewRetrievalMarketClientFakeAPI(t *testing.T, bal abi.TokenAmount) *RetrievalMarketClientFakeAPI {
	return &RetrievalMarketClientFakeAPI{
		Balance:          bal,
		WorkerAddr:       requireMakeTestFcAddr(t),
		Nonce:            rand.Uint64(),
		ExpectedLanes:    make(map[address.Address]int64),
		ExpectedPmtChans: make(map[address.Address]PmtChanEntry),
		ActualPmtChans:   make(map[address.Address]PmtChanEntry),
	}
}

// -------------- API METHODS

// AllocateLane mocks allocation of a new lane in a payment channel
func (rmFake *RetrievalMarketClientFakeAPI) AllocateLane(paychAddr address.Address) (int64, error) {
	lane, ok := rmFake.ExpectedLanes[paychAddr]
	if ok {
		rmFake.ExpectedLanes[paychAddr] = lane + 1
	}
	return lane, nil
}

func (rmFake *RetrievalMarketClientFakeAPI) CreatePaymentChannel(payer, payee address.Address) (paymentchannel.ChannelInfo, error) {
	panic("implement me")
}

func (rmFake *RetrievalMarketClientFakeAPI) UpdatePaymentChannel(paychAddr address.Address) error {
	panic("implement me")
}

// GetChannelInfo mocks getting payment channel info
// TODO: this should use the store's ChannelInfo struct
func (rmFake *RetrievalMarketClientFakeAPI) GetPaymentChannelInfo(paychAddr address.Address) (paymentchannel.ChannelInfo, error) {
	for payer, entry := range rmFake.ActualPmtChans {
		if entry.ChannelID == paychAddr {
			//Payee:    entry.Payee,
			//Amount:   types.NewAttoFIL(entry.FundsAvail.Int),
			//Redeemed: types.NewAttoFIL(entry.Redeemed.Int),

			pch := paymentchannel.ChannelInfo{
				Owner: payer,
				State: &paychActor.State{
					From:            payer,
					To:              entry.Payee,
					ToSend:          entry.FundsAvail,
					SettlingAt:      1,
					MinSettleHeight: 1,
				},
				Vouchers: nil,
			}

			return pch, nil
		}
	}
	return paymentchannel.ChannelInfo{}, errors.New("no such ChannelID")
}

// Wait mocks waiting for a message with a given CID to appear on chain, then actually calls
// the provided callback
func (rmFake *RetrievalMarketClientFakeAPI) Wait(_ context.Context, _ cid.Cid, cb func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error {
	if rmFake.WaitErr != nil {
		return rmFake.WaitErr
	}

	clientAddr := rmFake.ExpectedSignedMsg.Message.From
	rmFake.ActualPmtChans[clientAddr] = rmFake.ExpectedPmtChans[clientAddr]

	return cb(rmFake.ExpectedBlock, rmFake.ExpectedSignedMsg, rmFake.ExpectedMsgReceipt)
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
func (rmFake *RetrievalMarketClientFakeAPI) SignBytes(_ []byte, _ address.Address) (types.Signature, error) {
	return rmFake.Sig.Data, rmFake.SigErr
}

// Send mocks sending a message on chain
func (rmFake *RetrievalMarketClientFakeAPI) Send(ctx context.Context,
	from, to address.Address,
	value types.AttoFIL,
	gasPrice types.AttoFIL, gasLimit types.GasUnits,
	bcast bool,
	method types.MethodID,
	params interface{}) (out cid.Cid, pubErrCh chan error, err error) {
	rmFake.Nonce++

	if err != nil {
		return cid.Undef, nil, err
	}
	return rmFake.MsgSendCid, nil, rmFake.MsgSendErr
}

// ---------------  Testing methods

// StubMessageResponse sets up a message, message receipt and return value for a create payment
// channel message
func (rmFake *RetrievalMarketClientFakeAPI) StubMessageResponse(t *testing.T, from, to address.Address, value types.AttoFIL) {
	//params, err := abi.ToEncodedValues(to, uint64(1))

	//require.NoError(t, err)

	unsignedMsg := types.UnsignedMessage{
		To:         to,
		From:       from,
		CallSeqNum: 0,
		Value:      value,
		Method:     types.MethodID(builtin.MethodsInit.Exec),
		Params:     nil,
		GasPrice:   types.AttoFIL{},
		GasLimit:   0,
	}

	newAddr, err := address.NewIDAddress(rand.Uint64())
	require.NoError(t, err)
	rmFake.ExpectedPmtChans[from] = PmtChanEntry{
		ChannelID:  newAddr,
		FundsAvail: abi.TokenAmount{Int: value.Int},
		Redeemed:   abi.NewTokenAmount(0),
	}

	require.NoError(t, err)
	rmFake.ExpectedMsgReceipt = &vm.MessageReceipt{
		ExitCode:    0,
		ReturnValue: newAddr.Bytes(),
		GasUsed:     gas.Unit{},
	}

	mockSigner, _ := types.NewMockSignersAndKeyInfo(1)
	addr1 := mockSigner.Addresses[0]

	marshaled, err := unsignedMsg.Marshal()
	require.NoError(t, err)
	sig, err := mockSigner.SignBytes(marshaled, addr1)
	require.NoError(t, err)

	signature := &crypto.Signature{
		Type: crypto.SigTypeBLS,
		Data: sig,
	}
	rmFake.Sig = signature
	rmFake.ExpectedSignedMsg = &types.SignedMessage{
		Message:   unsignedMsg,
		Signature: sig,
	}
}

// requireMakeTestFcAddr generates a random ID addr for test
func requireMakeTestFcAddr(t *testing.T) address.Address {
	res, err := address.NewIDAddress(rand.Uint64())
	require.NoError(t, err)
	return res
}

var _ MgrAPI = &RetrievalMarketClientFakeAPI{}
var _ MsgSender = &RetrievalMarketClientFakeAPI{}
var _ MsgWaiter = &RetrievalMarketClientFakeAPI{}
var _ RetrievalSigner = &RetrievalMarketClientFakeAPI{}
var _ WalletAPI = &RetrievalMarketClientFakeAPI{}
