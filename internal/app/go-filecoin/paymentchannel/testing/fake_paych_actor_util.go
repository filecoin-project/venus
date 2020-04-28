package testing

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/support/mock"
	spect "github.com/filecoin-project/specs-actors/support/testing"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

// FakePaychActorUtil fulfils the MsgSender and MsgWaiter interfaces for a Manager
// via the specs_actors mock runtime. It executes paych.Actor exports directly.
type FakePaychActorUtil struct {
	t   *testing.T
	ctx context.Context
	*mock.Runtime
	*pcActorHarness
	PaychAddr, PaychIDAddr, Client, ClientID, Miner address.Address
	result                                          MsgResult
}

// NewFakePaychActorUtil intializes a FakePaychActorUtil
func NewFakePaychActorUtil(ctx context.Context, t *testing.T, paychBal abi.TokenAmount) *FakePaychActorUtil {
	fai := &FakePaychActorUtil{
		t:              t,
		ctx:            ctx,
		PaychAddr:      spect.NewActorAddr(t, "paychactor"),
		PaychIDAddr:    spect.NewIDAddr(t, 999),
		Client:         spect.NewActorAddr(t, "clientactor"),
		ClientID:       spect.NewIDAddr(t, 980),
		Miner:          spect.NewActorAddr(t, "mineractor"),
		pcActorHarness: new(pcActorHarness),
	}
	fai.constructPaychActor(paychBal)
	return fai
}

// constructPaychActor creates a mock.Runtime and constructs a payment channel harness + Actor
func (fai *FakePaychActorUtil) constructPaychActor(paychBal abi.TokenAmount) {
	builder := mock.NewBuilder(fai.ctx, fai.PaychAddr).
		WithBalance(paychBal, big.Zero()).
		WithEpoch(abi.ChainEpoch(42)).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID).
		WithActorType(fai.PaychIDAddr, builtin.AccountActorCodeID).
		WithActorType(fai.ClientID, builtin.AccountActorCodeID)

	fai.Runtime = builder.Build(fai.t)
	fai.Runtime.AddIDAddress(fai.PaychAddr, fai.PaychIDAddr)
	fai.Runtime.AddIDAddress(fai.Client, fai.ClientID)
	fai.pcActorHarness.constructAndVerify(fai.t, fai.Runtime, fai.Client, fai.PaychAddr)
}

// Send stubs a message Sender
func (fai *FakePaychActorUtil) Send(ctx context.Context,
	from, to address.Address,
	value types.AttoFIL,
	gasPrice types.AttoFIL,
	gasLimit gas.Unit,
	bcast bool,
	method abi.MethodNum,
	params interface{}) (out cid.Cid, pubErrCh chan error, err error) {

	return cid.Undef, nil, nil
}

// Wait stubs a message Waiter
func (fai *FakePaychActorUtil) Wait(_ context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error {
	require.Equal(fai.t, msgCid, fai.result.MsgCid)
	res := fai.result
	return cb(res.Block, res.Msg, res.Rcpt)
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
