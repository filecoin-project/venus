package consensus_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/consensus"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var keys = types.MustGenerateKeyInfo(2, 42)
var signer = types.NewMockSigner(keys)
var addresses = make([]address.Address, len(keys))

func init() {
	for i, k := range keys {
		addr, _ := k.Address()
		addresses[i] = addr
	}
}

func TestMessageValidator(t *testing.T) {
	tf.UnitTest(t)

	alice := addresses[0]
	bob := addresses[1]
	actor := newActor(t, 1000, 100)

	validator := consensus.NewDefaultMessageValidator()
	ctx := context.Background()

	t.Run("valid", func(t *testing.T) {
		msg := newMessage(t, alice, bob, 100, 5, 1, 0)
		assert.NoError(t, validator.Validate(ctx, msg, actor))
	})

	t.Run("invalid signature fails", func(t *testing.T) {
		msg := newMessage(t, alice, bob, 100, 5, 1, 0)
		msg.Signature = []byte{}
		assert.Errorf(t, validator.Validate(ctx, msg, actor), "signature")

	})

	t.Run("self send fails", func(t *testing.T) {
		msg := newMessage(t, alice, alice, 100, 5, 1, 0)
		assert.Errorf(t, validator.Validate(ctx, msg, actor), "self")
	})

	t.Run("non-account actor fails", func(t *testing.T) {
		badActor := newActor(t, 1000, 100)
		badActor.Code = types.CidFromString(t, "somecid")
		msg := newMessage(t, alice, bob, 100, 5, 1, 0)
		assert.Errorf(t, validator.Validate(ctx, msg, badActor), "account")
	})

	t.Run("negative value fails", func(t *testing.T) {
		msg := newMessage(t, alice, alice, 100, -5, 1, 0)
		assert.Errorf(t, validator.Validate(ctx, msg, actor), "negative")
	})

	t.Run("block gas limit fails", func(t *testing.T) {
		msg := newMessage(t, alice, bob, 100, 5, 1, uint64(types.BlockGasLimit)+1)
		assert.Errorf(t, validator.Validate(ctx, msg, actor), "block limit")
	})

	t.Run("can't cover value", func(t *testing.T) {
		msg := newMessage(t, alice, bob, 100, 2000, 1, 0) // lots of value
		assert.Errorf(t, validator.Validate(ctx, msg, actor), "funds")

		msg = newMessage(t, alice, bob, 100, 5, 100000, 200) // lots of expensive gas
		assert.Errorf(t, validator.Validate(ctx, msg, actor), "funds")
	})

	t.Run("low nonce", func(t *testing.T) {
		msg := newMessage(t, alice, bob, 99, 5, 1, 0)
		assert.Errorf(t, validator.Validate(ctx, msg, actor), "too low")
	})

	t.Run("high nonce", func(t *testing.T) {
		msg := newMessage(t, alice, bob, 101, 5, 1, 0)
		assert.Errorf(t, validator.Validate(ctx, msg, actor), "too high")
	})
}

func TestOutboundMessageValidator(t *testing.T) {
	tf.UnitTest(t)

	alice := addresses[0]
	bob := addresses[1]
	actor := newActor(t, 1000, 100)

	validator := consensus.NewOutboundMessageValidator()
	ctx := context.Background()

	t.Run("allows high nonce", func(t *testing.T) {
		msg := newMessage(t, alice, bob, 100, 5, 1, 0)
		assert.NoError(t, validator.Validate(ctx, msg, actor))
		msg = newMessage(t, alice, bob, 101, 5, 1, 0)
		assert.NoError(t, validator.Validate(ctx, msg, actor))
	})
}

func TestIngestionValidator(t *testing.T) {
	tf.UnitTest(t)

	alice := addresses[0]
	bob := addresses[1]
	act := newActor(t, 1000, 53)
	api := NewMockIngestionValidatorAPI()
	api.ActorAddr = alice
	api.Actor = act

	mpoolCfg := config.NewDefaultConfig().Mpool
	validator := consensus.NewIngestionValidator(api, mpoolCfg)
	ctx := context.Background()

	t.Run("Validates extreme nonce gaps", func(t *testing.T) {
		msg := newMessage(t, alice, bob, 100, 5, 1, 0)
		assert.NoError(t, validator.Validate(ctx, msg))

		highNonce := uint64(act.Nonce + mpoolCfg.MaxNonceGap + 10)
		msg = newMessage(t, alice, bob, highNonce, 5, 1, 0)
		err := validator.Validate(ctx, msg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too much greater than actor nonce")
	})

	t.Run("Actor not found is not an error", func(t *testing.T) {
		msg := newMessage(t, bob, alice, 0, 0, 1, 0)
		assert.NoError(t, validator.Validate(ctx, msg))
	})
}

func newActor(t *testing.T, balanceAF int, nonce uint64) *actor.Actor {
	actor, err := account.NewActor(attoFil(balanceAF))
	require.NoError(t, err)
	actor.Nonce = types.Uint64(nonce)
	return actor
}

func newMessage(t *testing.T, from, to address.Address, nonce uint64, valueAF int,
	gasPrice int64, gasLimit uint64) *types.SignedMessage {
	val, ok := types.NewAttoFILFromString(fmt.Sprintf("%d", valueAF), 10)
	require.True(t, ok, "invalid attofil")
	msg := types.NewMessage(
		from,
		to,
		nonce,
		val,
		"method",
		[]byte("params"),
	)
	signed, err := types.NewSignedMessage(*msg, signer, types.NewGasPrice(gasPrice), types.NewGasUnits(gasLimit))
	require.NoError(t, err)
	return signed
}

func attoFil(v int) types.AttoFIL {
	val, _ := types.NewAttoFILFromString(fmt.Sprintf("%d", v), 10)
	return val
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

// GetActor
func (api *FakeIngestionValidatorAPI) GetActor(ctx context.Context, a address.Address) (*actor.Actor, error) {
	if a == api.ActorAddr {
		return api.Actor, nil
	}
	return &actor.Actor{}, nil
}
