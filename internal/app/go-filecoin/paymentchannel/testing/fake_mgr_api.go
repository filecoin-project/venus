package testing

import (
	"context"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	initActor "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	spect "github.com/filecoin-project/specs-actors/support/testing"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

// FakePaymentChannelAPI mocks some needed APIs for a payment channel manager
type FakePaymentChannelAPI struct {
	t   *testing.T
	ctx context.Context

	Balance types.AttoFIL

	ActualWaitCid  cid.Cid
	ExpectedMsgCid cid.Cid
	ExpectedResult MsgResult
	ActualResult   MsgResult

	MsgSendErr error
	MsgWaitErr error
}

// MsgResult stores test message receipts
type MsgResult struct {
	Block         *block.Block
	Msg           *types.SignedMessage
	DecodedParams interface{}
	MsgCid        cid.Cid
	Rcpt          *vm.MessageReceipt
}

var msgRcptsUndef = MsgResult{}

// NewFakePaymentChannelAPI creates a new mock payment channel API
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
	return cb(f.ExpectedResult.Block, f.ExpectedResult.Msg, f.ExpectedResult.Rcpt)
}

// Send mocks sending a message on chain
func (f *FakePaymentChannelAPI) Send(_ context.Context,
	from, to address.Address,
	value types.AttoFIL,
	gasPrice types.AttoFIL,
	gasLimit gas.Unit,
	bcast bool,
	method abi.MethodNum,
	params interface{}) (out cid.Cid, pubErrCh chan error, err error) {

	if f.MsgSendErr != nil {
		return cid.Undef, nil, f.MsgSendErr
	}
	if f.ExpectedResult == msgRcptsUndef || f.ExpectedMsgCid == cid.Undef {
		f.t.Fatal("no message or no cid registered")
	}

	expMessage := f.ExpectedResult.Msg.Message
	require.Equal(f.t, f.ExpectedResult.DecodedParams, params)
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

// GenCreatePaychActorMessage sets up a message response, with desired exit code and block height
func GenCreatePaychActorMessage(
	t *testing.T,
	clientAccountAddr, minerAccountAddr, paychUniqueAddr address.Address,
	amt abi.TokenAmount,
	code exitcode.ExitCode,
	height uint64) (cid.Cid, MsgResult) {

	newcid := shared_testutil.GenerateCids(1)[0]

	msg := types.NewUnsignedMessage(clientAccountAddr, builtin.InitActorAddr, 1,
		types.NewAttoFIL(amt.Int), builtin.MethodsInit.Exec, []byte{})
	msg.GasPrice = types.NewAttoFILFromFIL(100)
	msg.GasLimit = gas.NewGas(300)

	params, err := paymentchannel.PaychActorCtorExecParamsFor(clientAccountAddr, minerAccountAddr)
	if err != nil {
		t.Fatal("could not construct send params")
	}
	msg.Params = requireEncode(t, &params)

	retVal := initActor.ExecReturn{
		IDAddress:     spect.NewIDAddr(t, rand.Uint64()), // IDAddress is currently unused
		RobustAddress: paychUniqueAddr,
	}

	emptySig := crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte{'0'}}
	return newcid, MsgResult{
		Block:         &block.Block{Height: abi.ChainEpoch(height)},
		Msg:           &types.SignedMessage{Message: *msg, Signature: emptySig},
		MsgCid:        newcid,
		Rcpt:          &vm.MessageReceipt{ExitCode: code, ReturnValue: requireEncode(t, &retVal)},
		DecodedParams: params,
	}
}

func requireEncode(t *testing.T, params interface{}) []byte {
	encodedParams, err := encoding.Encode(params)
	if err != nil {
		t.Fatal(err.Error())
	}
	return encodedParams
}

var _ paymentchannel.MsgSender = &FakePaymentChannelAPI{}
var _ paymentchannel.MsgWaiter = &FakePaymentChannelAPI{}
