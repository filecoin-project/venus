package paymentchannel

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	initActor "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
)

type FakePaymentChannelAPI struct {
	t   *testing.T
	ctx context.Context

	ActualWaitCid  cid.Cid
	ExpectedMsgCid cid.Cid
	ExpectedMsg    MsgReceipts
	ActualMsg      MsgReceipts

	MsgSendErr error
	MsgWaitErr error
}

type MsgReceipts struct {
	Block         *block.Block
	Msg           *types.SignedMessage
	DecodedParams interface{}
	MsgCid        cid.Cid
	Rcpt          *vm.MessageReceipt
}

var msgRcptsUndef = MsgReceipts{}

func NewFakePaymentChannelAPI(ctx context.Context, t *testing.T) *FakePaymentChannelAPI {
	return &FakePaymentChannelAPI{
		t:   t,
		ctx: ctx,
	}
}

// API methods

// Wait mocks waiting for a message to be mined
func (f *FakePaymentChannelAPI) Wait(_ context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error {
	if f.MsgWaitErr != nil {
		return f.MsgWaitErr
	}
	f.ActualWaitCid = msgCid
	return cb(f.ExpectedMsg.Block, f.ExpectedMsg.Msg, f.ExpectedMsg.Rcpt)
}

// Send mocks sending a message on chain
func (f *FakePaymentChannelAPI) Send(_ context.Context,
	from, to address.Address,
	value types.AttoFIL,
	gasPrice types.AttoFIL,
	gasLimit types.GasUnits,
	bcast bool,
	method abi.MethodNum,
	params interface{}) (out cid.Cid, pubErrCh chan error, err error) {

	if f.MsgSendErr != nil {
		return cid.Undef, nil, f.MsgSendErr
	}
	if f.ExpectedMsg == msgRcptsUndef || f.ExpectedMsgCid == cid.Undef {
		f.t.Fatal("no message or no cid registered")
	}

	expMessage := f.ExpectedMsg.Msg.Message
	require.Equal(f.t, f.ExpectedMsg.DecodedParams, params)
	require.Equal(f.t, expMessage.GasLimit, gasLimit)
	require.Equal(f.t, expMessage.GasPrice, gasPrice)
	require.Equal(f.t, expMessage.From, from)
	require.Equal(f.t, expMessage.To, to)
	require.Equal(f.t, expMessage.Value, value)
	require.Equal(f.t, expMessage.Method, method)
	require.True(f.t, bcast)
	return f.ExpectedMsgCid, nil, nil
}

// testing methods

// StubCreatePaychActorMessage sets up a message response, with desired exit code and block height
func (f *FakePaymentChannelAPI) StubCreatePaychActorMessage(clientAccountAddr, minerAccountAddr, paychIDAddr, paychUniqueAddr address.Address, method abi.MethodNum, code exitcode.ExitCode, height uint64) {

	newcid := shared_testutil.GenerateCids(1)[0]

	msg := types.NewUnsignedMessage(clientAccountAddr, builtin.InitActorAddr, 1, types.ZeroAttoFIL, method, []byte{})
	msg.GasPrice = defaultGasPrice
	msg.GasLimit = defaultGasLimit

	params, err := PaychActorCtorExecParamsFor(clientAccountAddr, minerAccountAddr)
	if err != nil {
		f.t.Fatal("could not construct send params")
	}
	msg.Params = f.requireEncode(&params)
	f.ExpectedMsgCid = newcid

	retVal := initActor.ExecReturn{IDAddress: paychIDAddr, RobustAddress: paychUniqueAddr}

	emptySig := crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte{'0'}}
	f.ExpectedMsg = MsgReceipts{
		Block:         &block.Block{Height: abi.ChainEpoch(height)},
		Msg:           &types.SignedMessage{Message: *msg, Signature: emptySig},
		MsgCid:        newcid,
		Rcpt:          &vm.MessageReceipt{ExitCode: code, ReturnValue: f.requireEncode(&retVal)},
		DecodedParams: params,
	}
}

// Verify compares expected and actual results
func (f *FakePaymentChannelAPI) Verify() {
	assert.True(f.t, f.ExpectedMsgCid.Equals(f.ActualWaitCid))
}

func (f *FakePaymentChannelAPI) requireEncode(params interface{}) []byte {
	encodedParams, err := encoding.Encode(params)
	if err != nil {
		f.t.Fatal(err.Error())
	}
	return encodedParams
}

var _ MsgSender = &FakePaymentChannelAPI{}
var _ MsgWaiter = &FakePaymentChannelAPI{}
