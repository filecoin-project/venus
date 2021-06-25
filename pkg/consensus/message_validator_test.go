package consensus_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/venus/pkg/constants"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/state"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/pkg/types"
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

func TestBLSSignatureValidationConfiguration(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	// create bls address
	pubKey := bls.PrivateKeyPublicKey(bls.PrivateKeyGenerate())
	from, err := address.NewBLSAddress(pubKey[:])
	require.NoError(t, err)

	msg := types.NewMeteredMessage(from, addresses[1], 0, types.ZeroFIL, methodID, []byte("params"), types.NewGasFeeCap(1), types.NewGasPremium(1), 300)
	mmsgCid := msg.Cid()

	var signer = types.NewMockSigner(keys)
	signer.AddrKeyInfo[msg.From] = keys[0]
	sig, err := signer.SignBytes(ctx, mmsgCid.Bytes(), msg.From)
	require.NoError(t, err)
	unsigned := &types.SignedMessage{Message: *msg, Signature: *sig}

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
	actor.Nonce = nonce
	return actor
}

func newMessage(t *testing.T, from, to address.Address, nonce uint64, valueAF int,
	gasPrice int64, gasLimit int64) *types.UnsignedMessage {
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
	Block     *types.BlockHeader
	ActorAddr address.Address
	Actor     *types.Actor
}

// NewMockIngestionValidatorAPI creates a new FakeIngestionValidatorAPI.
func NewMockIngestionValidatorAPI() *FakeIngestionValidatorAPI {
	block := mockBlock()
	block.Height = 10
	return &FakeIngestionValidatorAPI{
		Actor: &types.Actor{},
		Block: block,
	}
}

func (api *FakeIngestionValidatorAPI) GetHead() *types.TipSet {
	ts, _ := types.NewTipSet(api.Block)
	return ts
}

func (api *FakeIngestionValidatorAPI) GetTipSet(key types.TipSetKey) (*types.TipSet, error) {
	return types.NewTipSet(api.Block)
}

func (api *FakeIngestionValidatorAPI) GetActorAt(ctx context.Context, key *types.TipSet, a address.Address) (*types.Actor, error) {
	if a == api.ActorAddr {
		return api.Actor, nil
	}
	return &types.Actor{
		Balance: abi.NewTokenAmount(0),
	}, nil
}

func (api *FakeIngestionValidatorAPI) AccountView(ts *types.TipSet) (state.AccountView, error) {
	return &state.FakeStateView{}, nil
}
