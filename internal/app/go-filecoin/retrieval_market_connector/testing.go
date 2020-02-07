package retrieval_market_connector

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	gfm_types "github.com/filecoin-project/go-fil-markets/shared/types"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
)

type RetrievalMarketClientFakeAPI struct {
	AllocateLaneErr error

	ExpectedLanes    map[address.Address]uint64       // mock payment broker lane store
	ExpectedPmtChans map[address.Address]PmtChanEntry // mock payment broker's payment channel store
	ActualPmtChans   map[address.Address]PmtChanEntry // to check that the payment channels were created

	PayChBalanceErr error

	Balance       tokenamount.TokenAmount
	BalanceErr    error
	WorkerAddr    address.Address
	WorkerAddrErr error
	Nonce         uint64
	NonceErr      error
	Sig           *gfm_types.Signature
	SigErr        error
	MsgSendCid    cid.Cid
	MsgSendErr    error
	WaitErr       error

	ExpectedBlock      *block.Block
	ExpectedMsgReceipt *types.MessageReceipt
	ExpectedSignedMsg  *types.SignedMessage
}
// PmtChanEntry is a record of a created payment channel with funds available.
type PmtChanEntry struct {
	Payee      address.Address
	Redeemed   tokenamount.TokenAmount
	ChannelID  address.Address
	FundsAvail tokenamount.TokenAmount
}

func NewRetrievalMarketClientFakeAPI(t *testing.T, bal tokenamount.TokenAmount) *RetrievalMarketClientFakeAPI {
	return &RetrievalMarketClientFakeAPI{
		Balance:          bal,
		WorkerAddr:       requireMakeTestFcAddr(t),
		Nonce:            rand.Uint64(),
		ExpectedLanes:    make(map[address.Address]uint64),
		ExpectedPmtChans: make(map[address.Address]PmtChanEntry),
		ActualPmtChans:   make(map[address.Address]PmtChanEntry),
	}
}

func (rmFake *RetrievalMarketClientFakeAPI) GetChannelInfo(_ context.Context, paymentChannel address.Address) (address.Address, ChannelInfo, error) {
	for payer, entry := range rmFake.ActualPmtChans {
		if entry.ChannelID == paymentChannel {
			pch := ChannelInfo{
				Payee:         entry.Payee,
				Amount:         types.NewAttoFIL(entry.FundsAvail.Int),
				Redeemed: types.NewAttoFIL(entry.Redeemed.Int),
			}

			return payer, pch, nil
		}
	}
	return address.Undef, ChannelInfo{}, errors.New("no such ChannelID")
}

func (rmFake *RetrievalMarketClientFakeAPI) Wait(_ context.Context, _ cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	if rmFake.WaitErr != nil {
		return rmFake.WaitErr
	}

	clientAddr := rmFake.ExpectedSignedMsg.Message.From
	rmFake.ActualPmtChans[clientAddr] = rmFake.ExpectedPmtChans[clientAddr]

	cb(rmFake.ExpectedBlock, rmFake.ExpectedSignedMsg, rmFake.ExpectedMsgReceipt)
	return nil
}

func (rmFake *RetrievalMarketClientFakeAPI) GetBalance(_ context.Context, _ address.Address) (types.AttoFIL, error) {
	return types.NewAttoFIL(rmFake.Balance.Int), rmFake.BalanceErr
}
func (rmFake *RetrievalMarketClientFakeAPI) GetWorkerAddress(_ context.Context, _ address.Address, _ block.TipSetKey) (address.Address, error) {
	return rmFake.WorkerAddr, rmFake.WorkerAddrErr
}
func (rmFake *RetrievalMarketClientFakeAPI) NextNonce(_ context.Context, _ address.Address) (uint64, error) {
	rmFake.Nonce++
	return rmFake.Nonce, rmFake.NonceErr
}

func (rmFake *RetrievalMarketClientFakeAPI) SignBytes(_ []byte, _ address.Address) (types.Signature, error) {
	return rmFake.Sig.Data, rmFake.SigErr
}

func (rmFake *RetrievalMarketClientFakeAPI) Send(_ context.Context, _, _ address.Address, _ types.AttoFIL,
	gasPrice types.AttoFIL, gasLimit types.GasUnits, bcast bool, method types.MethodID, params ...interface{}) (out cid.Cid, pubErrCh chan error, err error) {
	rmFake.Nonce++

	if err != nil {
		return cid.Undef, nil, err
	}
	return rmFake.MsgSendCid, nil, rmFake.MsgSendErr
}

// GetPaymentChannelIDByPayee searches for a payment channel for a payer + Payee.
// It does not assume the payment channel has been created. If not found, returns
// 0 channel ID and nil.
func (rmFake *RetrievalMarketClientFakeAPI) GetChannel(ctx context.Context, payer, _ address.Address) (address.Address, error) {
	entry, ok := rmFake.ActualPmtChans[payer]
	if !ok {
		return address.Undef, nil
	}
	// assuming only one client for test purposes
	return entry.ChannelID, nil
}

func (rmFake *RetrievalMarketClientFakeAPI) AllocateLane(_ context.Context, _ address.Address, chid address.Address) (uint64, error) {
	lane, ok := rmFake.ExpectedLanes[chid]
	if ok {
		rmFake.ExpectedLanes[chid] = lane+1
	}
	return lane, nil
}

func (rmFake *RetrievalMarketClientFakeAPI) StubMessageResponse(t *testing.T, from, to address.Address, value types.AttoFIL) {
	params, err := abi.ToEncodedValues(to, uint64(1))
	require.NoError(t, err)

	unsignedMsg := types.UnsignedMessage{
		To:         to,
		From:       from,
		CallSeqNum: 0,
		Value:      value,
		Method:     CreatePaymentChannelMethod,
		Params:     params,
		GasPrice:   types.AttoFIL{},
		GasLimit:   0,
	}

	newAddr, err := address.NewIDAddress(rand.Uint64())
	require.NoError(t, err)
	rmFake.ExpectedPmtChans[from] = PmtChanEntry{
		ChannelID:  newAddr,
		FundsAvail: tokenamount.TokenAmount{Int: value.AsBigInt()},
		Redeemed:   tokenamount.FromInt(0),
	}

	require.NoError(t, err)
	rmFake.ExpectedMsgReceipt = &types.MessageReceipt{
		ExitCode:   0,
		Return:     [][]byte{newAddr.Bytes()},
		GasAttoFIL: types.AttoFIL{},
	}

	mockSigner, _ := types.NewMockSignersAndKeyInfo(1)
	addr1 := mockSigner.Addresses[0]

	marshaled, err := unsignedMsg.Marshal()
	require.NoError(t, err)
	sig, err := mockSigner.SignBytes(marshaled, addr1)
	require.NoError(t, err)
	signature := &gfm_types.Signature{
		Type: gfm_types.KTBLS,
		Data: sig,
	}
	rmFake.Sig = signature
	rmFake.ExpectedSignedMsg = &types.SignedMessage{
		Message:   unsignedMsg,
		Signature: sig,
	}
}

func requireMakeTestFcAddr(t *testing.T) address.Address {
	res, err := address.NewIDAddress(rand.Uint64())
	require.NoError(t, err)
	return res
}
