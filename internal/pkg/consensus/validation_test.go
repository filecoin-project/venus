package consensus_test

import (
	"context"
	"fmt"
	"github.com/filecoin-project/venus/internal/pkg/constants"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/consensus"
	"github.com/filecoin-project/venus/internal/pkg/enccid"
	"github.com/filecoin-project/venus/internal/pkg/state"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

var keys = types.MustGenerateKeyInfo(2, 42)
var addresses = make([]address.Address, len(keys))

var methodID = abi.MethodNum(21231)

func init() {
	for i, k := range keys {
		addr, _ := k.Address()
		addresses[i] = addr
	}
}

func TestMessagePenaltyChecker(t *testing.T) {
	tf.UnitTest(t)

	alice := addresses[0]
	bob := addresses[1]
	actor := newActor(t, 1000, 100)
	api := NewMockIngestionValidatorAPI()
	api.ActorAddr = alice
	api.Actor = actor

	checker := consensus.NewMessagePenaltyChecker(api)
	ctx := context.Background()

	t.Run("valid", func(t *testing.T) {
		msg := newMessage(t, alice, bob, 100, 5, 1, 0)
		assert.NoError(t, checker.PenaltyCheck(ctx, msg))
	})

	t.Run("non-account actor fails", func(t *testing.T) {
		badActor := newActor(t, 1000, 100)
		badActor.Code = enccid.NewCid(types.CidFromString(t, "somecid"))
		msg := newMessage(t, alice, bob, 100, 5, 1, 0)
		api := NewMockIngestionValidatorAPI()
		api.ActorAddr = alice
		api.Actor = badActor
		checker := consensus.NewMessagePenaltyChecker(api)
		assert.Errorf(t, checker.PenaltyCheck(ctx, msg), "account")
	})

	t.Run("can't cover value", func(t *testing.T) {
		msg := newMessage(t, alice, bob, 100, 2000, 1, 0) // lots of value
		assert.Errorf(t, checker.PenaltyCheck(ctx, msg), "funds")

		msg = newMessage(t, alice, bob, 100, 5, 100000, 200) // lots of expensive gas
		assert.Errorf(t, checker.PenaltyCheck(ctx, msg), "funds")
	})

	t.Run("low nonce", func(t *testing.T) {
		msg := newMessage(t, alice, bob, 99, 5, 1, 0)
		assert.Errorf(t, checker.PenaltyCheck(ctx, msg), "too low")
	})

	t.Run("high nonce", func(t *testing.T) {
		msg := newMessage(t, alice, bob, 101, 5, 1, 0)
		assert.Errorf(t, checker.PenaltyCheck(ctx, msg), "too high")
	})
}

func TestBLSSignatureValidationConfiguration(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	// create bls address
	pubKey := bls.PrivateKeyPublicKey(bls.PrivateKeyGenerate())
	from, err := address.NewBLSAddress(pubKey[:])
	require.NoError(t, err)

	msg := types.NewMeteredMessage(from, addresses[1], 0, types.ZeroAttoFIL, methodID, []byte("params"), types.NewGasFeeCap(1), types.NewGasPremium(1), types.NewGas(300))
	mmsgCid, err := msg.Cid()
	require.NoError(t, err)

	var signer = types.NewMockSigner(keys)
	signer.AddrKeyInfo[msg.From] = keys[0]
	sig, err := signer.SignBytes(ctx, mmsgCid.Bytes(), msg.From)
	require.NoError(t, err)
	unsigned := &types.SignedMessage{Message: *msg, Signature: sig}

	actor := newActor(t, 1000, 0)

	t.Run("syntax validator does not ignore missing signature", func(t *testing.T) {
		api := NewMockIngestionValidatorAPI()
		api.ActorAddr = from
		api.Actor = actor

		validator := consensus.NewMessageSignatureValidator(api)

		err := validator.Validate(ctx, unsigned)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid signature")
	})
}

func TestMessageSyntaxValidator(t *testing.T) {
	tf.UnitTest(t)
	var signer = types.NewMockSigner(keys)
	alice := addresses[0]
	bob := addresses[1]

	validator := consensus.NewMessageSyntaxValidator()
	ctx := context.Background()

	t.Run("Actor not found is not an error", func(t *testing.T) {
		msg, err := types.NewSignedMessage(ctx, *newMessage(t, bob, alice, 0, 0, 1, 5000), signer)
		require.NoError(t, err)
		assert.NoError(t, validator.ValidateSignedMessageSyntax(ctx, msg))
	})

	t.Run("self send passes", func(t *testing.T) {
		msg, err := types.NewSignedMessage(ctx, *newMessage(t, alice, alice, 100, 5, 1, 5000), signer)
		require.NoError(t, err)
		assert.NoError(t, validator.ValidateSignedMessageSyntax(ctx, msg), "self")
	})

	t.Run("negative value fails", func(t *testing.T) {
		msg, err := types.NewSignedMessage(ctx, *newMessage(t, alice, alice, 100, -5, 1, 5000), signer)
		require.NoError(t, err)
		assert.Errorf(t, validator.ValidateSignedMessageSyntax(ctx, msg), "negative")
	})

	t.Run("block gas limit fails", func(t *testing.T) {
		msg, err := types.NewSignedMessage(ctx, *newMessage(t, alice, bob, 100, 5, 1, constants.BlockGasLimit+1), signer)
		require.NoError(t, err)
		assert.Errorf(t, validator.ValidateSignedMessageSyntax(ctx, msg), "block limit")
	})

}

func newActor(t *testing.T, balanceAF int, nonce uint64) *types.Actor {
	actor := types.NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(int64(balanceAF)), cid.Undef)
	actor.CallSeqNum = nonce
	return actor
}

func newMessage(t *testing.T, from, to address.Address, nonce uint64, valueAF int,
	gasPrice int64, gasLimit types.Unit) *types.UnsignedMessage {
	val, ok := types.NewAttoFILFromString(fmt.Sprintf("%d", valueAF), 10)
	require.True(t, ok, "invalid attofil")
	return types.NewMeteredMessage(
		from,
		to,
		nonce,
		val,
		methodID,
		[]byte("params"),
		types.NewGasFeeCap(gasPrice),
		types.NewGasPremium(1),
		gasLimit,
	)
}

// FakeIngestionValidatorAPI provides a latest state
type FakeIngestionValidatorAPI struct {
	Block     *block.Block
	ActorAddr address.Address
	Actor     *types.Actor
}

// NewMockIngestionValidatorAPI creates a new FakeIngestionValidatorAPI.
func NewMockIngestionValidatorAPI() *FakeIngestionValidatorAPI {
	block := &block.Block{
		Height: 10,
	}
	return &FakeIngestionValidatorAPI{
		Actor: &types.Actor{},
		Block: block,
	}
}

func (api *FakeIngestionValidatorAPI) Head() block.TipSetKey {
	return block.NewTipSetKey(api.Block.Cid())
}

func (api *FakeIngestionValidatorAPI) GetTipSet(key block.TipSetKey) (*block.TipSet, error) {
	return block.NewTipSet(api.Block)
}

func (api *FakeIngestionValidatorAPI) GetActorAt(ctx context.Context, key block.TipSetKey, a address.Address) (*types.Actor, error) {
	if a == api.ActorAddr {
		return api.Actor, nil
	}
	return &types.Actor{
		Balance: abi.NewTokenAmount(0),
	}, nil
}

func (api *FakeIngestionValidatorAPI) AccountStateView(baseKey block.TipSetKey, height abi.ChainEpoch) (state.AccountStateView, error) {
	return &state.FakeStateView{}, nil
}
