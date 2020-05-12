package consensus_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/ipfs/go-cid"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		badActor.Code = e.NewCid(types.CidFromString(t, "somecid"))
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

	msg := types.NewMeteredMessage(from, addresses[1], 0, types.ZeroAttoFIL, methodID, []byte("params"), types.NewGasPrice(1), gas.NewGas(300))
	unsigned := &types.SignedMessage{Message: *msg}
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
		msg, err := types.NewSignedMessage(ctx, *newMessage(t, alice, bob, 100, 5, 1, types.BlockGasLimit+1), signer)
		require.NoError(t, err)
		assert.Errorf(t, validator.ValidateSignedMessageSyntax(ctx, msg), "block limit")
	})

}

func newActor(t *testing.T, balanceAF int, nonce uint64) *actor.Actor {
	actor := actor.NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(int64(balanceAF)), cid.Undef)
	actor.CallSeqNum = nonce
	return actor
}

func newMessage(t *testing.T, from, to address.Address, nonce uint64, valueAF int,
	gasPrice int64, gasLimit gas.Unit) *types.UnsignedMessage {
	val, ok := types.NewAttoFILFromString(fmt.Sprintf("%d", valueAF), 10)
	require.True(t, ok, "invalid attofil")
	return types.NewMeteredMessage(
		from,
		to,
		nonce,
		val,
		methodID,
		[]byte("params"),
		types.NewGasPrice(gasPrice),
		gasLimit,
	)
}

// FakeIngestionValidatorAPI provides a latest state
type FakeIngestionValidatorAPI struct {
	ActorAddr address.Address
	Actor     *actor.Actor
}

// NewMockIngestionValidatorAPI creates a new FakeIngestionValidatorAPI.
func NewMockIngestionValidatorAPI() *FakeIngestionValidatorAPI {
	return &FakeIngestionValidatorAPI{Actor: &actor.Actor{}}
}

func (api *FakeIngestionValidatorAPI) Head() block.TipSetKey {
	return block.NewTipSetKey()
}

func (api *FakeIngestionValidatorAPI) GetActorAt(ctx context.Context, key block.TipSetKey, a address.Address) (*actor.Actor, error) {
	if a == api.ActorAddr {
		return api.Actor, nil
	}
	return &actor.Actor{
		Balance: abi.NewTokenAmount(0),
	}, nil
}

func (api *FakeIngestionValidatorAPI) AccountStateView(baseKey block.TipSetKey) (state.AccountStateView, error) {
	return &state.FakeStateView{}, nil
}
