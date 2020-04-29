package testing

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/filecoin-project/specs-actors/support/mock"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

// FakePaychActorUtil fulfils the MsgSender and MsgWaiter interfaces for a Manager
// via the specs_actors mock runtime. It executes paych.Actor exports directly.
type FakePaychActorUtil struct {
	*testing.T
	ctx context.Context
	*mock.Runtime
	*pcActorHarness
	PaychAddr, PaychIDAddr, Client, ClientID, Miner address.Address
	SendErr                                         error
	result                                          MsgResult
}

// ConstructPaychActor creates a mock.Runtime and constructs a payment channel harness + Actor
func (fai *FakePaychActorUtil) ConstructPaychActor(t *testing.T, paychBal abi.TokenAmount) {
	versig := func(sig crypto.Signature, signer address.Address, plaintext []byte) error {
		return nil
	}
	hasher := func(data []byte) [32]byte { return [32]byte{} }

	builder := mock.NewBuilder(fai.ctx, fai.PaychAddr).
		WithBalance(paychBal, big.Zero()).
		WithEpoch(abi.ChainEpoch(42)).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID).
		WithActorType(fai.PaychIDAddr, builtin.AccountActorCodeID).
		WithActorType(fai.ClientID, builtin.AccountActorCodeID).
		WithVerifiesSig(versig).
		WithHasher(hasher)

	fai.T = t
	fai.Runtime = builder.Build(fai.T)
	fai.Runtime.AddIDAddress(fai.PaychAddr, fai.PaychIDAddr)
	fai.Runtime.AddIDAddress(fai.Client, fai.ClientID)
	fai.pcActorHarness = new(pcActorHarness)
	fai.pcActorHarness.constructAndVerify(fai.T, fai.Runtime, fai.Client, fai.PaychAddr)
}

// Send stubs a message Sender
func (fai *FakePaychActorUtil) Send(ctx context.Context,
	from, to address.Address,
	value types.AttoFIL,
	gasPrice types.AttoFIL,
	gasLimit gas.Unit,
	bcast bool,
	method abi.MethodNum,
	params interface{}) (mcid cid.Cid, pubErrCh chan error, err error) {

	err = fai.SendErr

	if fai.result != msgRcptsUndef {
		mcid = fai.result.MsgCid
	}

	fai.doSend(value)
	return mcid, pubErrCh, fai.SendErr
}

// Wait stubs a message Waiter
func (fai *FakePaychActorUtil) Wait(_ context.Context, _ cid.Cid, cb func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error {
	res := fai.result
	return cb(res.Block, res.Msg, res.Rcpt)
}

// StubSendFundsMessage sets expectations for a message that just sends funds to the actor
func (fai *FakePaychActorUtil) StubSendFundsResponse(from address.Address, amt abi.TokenAmount, code exitcode.ExitCode, height int64) cid.Cid {
	newCID := shared_testutil.GenerateCids(1)[0]

	msg := types.NewUnsignedMessage(from, fai.PaychAddr, 1, amt, builtin.MethodSend, []byte{})
	msg.GasPrice = abi.NewTokenAmount(100)
	msg.GasLimit = gas.NewGas(5000)

	emptySig := crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte{'0'}}
	fai.result = MsgResult{
		Block:         &block.Block{Height: abi.ChainEpoch(height)},
		Msg:           &types.SignedMessage{Message: *msg, Signature: emptySig},
		DecodedParams: nil,
		MsgCid:        newCID,
		Rcpt:          &vm.MessageReceipt{ExitCode: code},
	}
	return newCID
}

func (fai *FakePaychActorUtil) doSend(amt abi.TokenAmount) {
	require.Equal(fai, amt, fai.result.Msg.Message.Value)
	fai.Runtime.SetReceived(amt)
}

type pcActorHarness struct {
	paych.Actor
	t testing.TB
}

func (h *pcActorHarness) constructAndVerify(t *testing.T, rt *mock.Runtime, sender, receiver address.Address) {
	params := &paych.ConstructorParams{To: receiver, From: sender}
	rt.ExpectValidateCallerType(builtin.InitActorCodeID)
	ret := rt.Call(h.Actor.Constructor, params)
	assert.Nil(h.t, ret)
	rt.Verify()
}
