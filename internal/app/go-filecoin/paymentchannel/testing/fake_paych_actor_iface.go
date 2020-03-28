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
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

type FakePaychActorIface struct {
	t   *testing.T
	ctx context.Context
	*mock.Runtime
	*pcActorHarness
	PaychAddr, PaychIDAddr, Client, ClientID, Miner address.Address
	result                                          MsgResult
}

func NewFakePaychActorIface(t *testing.T, ctx context.Context, paychBal abi.TokenAmount) *FakePaychActorIface {
	fai := &FakePaychActorIface{
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

func (fai *FakePaychActorIface) constructPaychActor(paychBal abi.TokenAmount) {
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

	fai.Runtime = builder.Build(fai.t)
	fai.Runtime.AddIDAddress(fai.PaychAddr, fai.PaychIDAddr)
	fai.Runtime.AddIDAddress(fai.Client, fai.ClientID)
	fai.pcActorHarness.constructAndVerify(fai.t, fai.Runtime, fai.Client, fai.PaychAddr)
}

func (fai *FakePaychActorIface) Send(ctx context.Context,
	from, to address.Address,
	value types.AttoFIL,
	gasPrice types.AttoFIL,
	gasLimit gas.Unit,
	bcast bool,
	method abi.MethodNum,
	params interface{}) (out cid.Cid, pubErrCh chan error, err error) {

	return cid.Undef, nil, nil
}

func (fai *FakePaychActorIface) Wait(_ context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error {
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
