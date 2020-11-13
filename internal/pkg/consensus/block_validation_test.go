package consensus_test

import (
	"context"
	fbig "github.com/filecoin-project/go-state-types/big"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/clock"
	"github.com/filecoin-project/venus/internal/pkg/consensus"
	"github.com/filecoin-project/venus/internal/pkg/crypto"
	e "github.com/filecoin-project/venus/internal/pkg/enccid"
	"github.com/filecoin-project/venus/internal/pkg/state"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

func TestBlockValidHeaderSemantic(t *testing.T) {
	tf.UnitTest(t)

	blockTime := clock.DefaultEpochDuration
	ts := time.Unix(1234567890, 0)
	genTime := ts
	mclock := clock.NewChainClockFromClock(uint64(genTime.Unix()), blockTime, clock.DefaultPropagationDelay, clock.NewFake(ts))
	ctx := context.Background()

	validator := consensus.NewDefaultBlockValidator(mclock, nil, nil)

	t.Run("reject block with same height as parents", func(t *testing.T) {
		// passes with valid height
		c := &block.Block{Height: 2, Timestamp: uint64(ts.Add(blockTime).Unix())}
		p := &block.Block{Height: 1, Timestamp: uint64(ts.Unix())}
		parents := consensus.RequireNewTipSet(require.New(t), p)
		require.NoError(t, validator.ValidateHeaderSemantic(ctx, c, parents))

		// invalidate parent by matching child height
		p = &block.Block{Height: 2, Timestamp: uint64(ts.Unix())}
		parents = consensus.RequireNewTipSet(require.New(t), p)

		err := validator.ValidateHeaderSemantic(ctx, c, parents)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid height")
	})
}

func TestBlockValidMessageSemantic(t *testing.T) {
	tf.UnitTest(t)

	blockTime := clock.DefaultEpochDuration
	ts := time.Unix(1234567890, 0)
	genTime := ts
	mclock := clock.NewChainClockFromClock(uint64(genTime.Unix()), blockTime, clock.DefaultPropagationDelay, clock.NewFake(ts))
	ctx := context.Background()

	c := &block.Block{Height: 2, Timestamp: uint64(ts.Add(blockTime).Unix())}
	p := &block.Block{Height: 1, Timestamp: uint64(ts.Unix())}
	parents := consensus.RequireNewTipSet(require.New(t), p)

	msg0 := &types.UnsignedMessage{From: address.TestAddress, To: address.TestAddress2, CallSeqNum: 1, Value: fbig.NewInt(0), GasFeeCap: fbig.NewInt(0), GasPremium: fbig.NewInt(0), GasLimit: 1000000}
	msg1 := &types.UnsignedMessage{From: address.TestAddress, To: address.TestAddress2, CallSeqNum: 2, Value: fbig.NewInt(0), GasFeeCap: fbig.NewInt(0), GasPremium: fbig.NewInt(0), GasLimit: 1000000}
	msg2 := &types.UnsignedMessage{From: address.TestAddress, To: address.TestAddress2, CallSeqNum: 3, Value: fbig.NewInt(0), GasFeeCap: fbig.NewInt(0), GasPremium: fbig.NewInt(0), GasLimit: 1000000}
	msg3 := &types.UnsignedMessage{From: address.TestAddress, To: address.TestAddress2, CallSeqNum: 4, Value: fbig.NewInt(0), GasFeeCap: fbig.NewInt(0), GasPremium: fbig.NewInt(0), GasLimit: 1000000}

	t.Run("rejects block with message from missing actor", func(t *testing.T) {
		validator := consensus.NewDefaultBlockValidator(mclock, &fakeMsgSource{
			blsMessages: []*types.UnsignedMessage{msg1},
		}, &fakeChainState{
			err: blockstore.ErrNotFound,
		})

		err := validator.ValidateMessagesSemantic(ctx, c, parents.Key())
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")
	})

	t.Run("rejects block with message from non-account actor", func(t *testing.T) {
		actor := newActor(t, 0, 2)

		// set invalid code
		actor.Code = e.NewCid(builtin.RewardActorCodeID)

		validator := consensus.NewDefaultBlockValidator(mclock, &fakeMsgSource{
			blsMessages: []*types.UnsignedMessage{msg1},
		}, &fakeChainState{
			actor: actor,
		})

		err := validator.ValidateMessagesSemantic(ctx, c, parents.Key())
		require.Error(t, err)
		require.Contains(t, err.Error(), "Sender must be an account actor")
	})

	t.Run("accepts block with bls messages in monotonic sequence", func(t *testing.T) {
		validator := consensus.NewDefaultBlockValidator(mclock, &fakeMsgSource{
			blsMessages: []*types.UnsignedMessage{msg1, msg2, msg3},
		}, &fakeChainState{
			actor: newActor(t, 0, 2),
		})

		err := validator.ValidateMessagesSemantic(ctx, c, parents.Key())
		require.NoError(t, err)
	})

	t.Run("accepts block with secp messages in monotonic sequence", func(t *testing.T) {
		validator := consensus.NewDefaultBlockValidator(mclock, &fakeMsgSource{
			secpMessages: []*types.SignedMessage{{Message: *msg1}, {Message: *msg2}, {Message: *msg3}},
		}, &fakeChainState{
			actor: newActor(t, 0, 2),
		})

		err := validator.ValidateMessagesSemantic(ctx, c, parents.Key())
		require.NoError(t, err)
	})

	t.Run("rejects block with messages out of order", func(t *testing.T) {
		validator := consensus.NewDefaultBlockValidator(mclock, &fakeMsgSource{
			blsMessages: []*types.UnsignedMessage{msg1, msg3, msg2},
		}, &fakeChainState{
			actor: newActor(t, 0, 2),
		})

		err := validator.ValidateMessagesSemantic(ctx, c, parents.Key())
		require.Error(t, err)
		require.Contains(t, err.Error(), "wrong nonce")
	})

	t.Run("rejects block with gaps", func(t *testing.T) {
		validator := consensus.NewDefaultBlockValidator(mclock, &fakeMsgSource{
			blsMessages: []*types.UnsignedMessage{msg1, msg3},
		}, &fakeChainState{
			actor: newActor(t, 0, 2),
		})

		err := validator.ValidateMessagesSemantic(ctx, c, parents.Key())
		require.Error(t, err)
		require.Contains(t, err.Error(), "wrong nonce")
	})

	t.Run("rejects block with bls message with nonce too low", func(t *testing.T) {
		validator := consensus.NewDefaultBlockValidator(mclock, &fakeMsgSource{
			blsMessages: []*types.UnsignedMessage{msg0},
		}, &fakeChainState{
			actor: newActor(t, 0, 2),
		})

		err := validator.ValidateMessagesSemantic(ctx, c, parents.Key())
		require.Error(t, err)
		require.Contains(t, err.Error(), "wrong nonce")
	})

	t.Run("rejects block with secp message with nonce too low", func(t *testing.T) {
		validator := consensus.NewDefaultBlockValidator(mclock, &fakeMsgSource{
			secpMessages: []*types.SignedMessage{{Message: *msg0}},
		}, &fakeChainState{
			actor: newActor(t, 0, 2),
		})

		err := validator.ValidateMessagesSemantic(ctx, c, parents.Key())
		require.Error(t, err)
		require.Contains(t, err.Error(), "wrong nonce")
	})

	t.Run("rejects block with message too high", func(t *testing.T) {
		validator := consensus.NewDefaultBlockValidator(mclock, &fakeMsgSource{
			blsMessages: []*types.UnsignedMessage{msg2},
		}, &fakeChainState{
			actor: newActor(t, 0, 2),
		})

		err := validator.ValidateMessagesSemantic(ctx, c, parents.Key())
		require.Error(t, err)
		require.Contains(t, err.Error(), "wrong nonce")
	})

	t.Run("rejects secp message < bls messages", func(t *testing.T) {
		validator := consensus.NewDefaultBlockValidator(mclock, &fakeMsgSource{
			secpMessages: []*types.SignedMessage{{Message: *msg1}},
			blsMessages:  []*types.UnsignedMessage{msg2},
		}, &fakeChainState{
			actor: newActor(t, 0, 2),
		})

		err := validator.ValidateMessagesSemantic(ctx, c, parents.Key())
		require.Error(t, err)
		require.Contains(t, err.Error(), "wrong nonce")
	})

	t.Run("accepts bls message < secp messages", func(t *testing.T) {
		validator := consensus.NewDefaultBlockValidator(mclock, &fakeMsgSource{
			blsMessages:  []*types.UnsignedMessage{msg1},
			secpMessages: []*types.SignedMessage{{Message: *msg2}},
		}, &fakeChainState{
			actor: newActor(t, 0, 2),
		})

		err := validator.ValidateMessagesSemantic(ctx, c, parents.Key())
		require.NoError(t, err)
	})
}

func TestMismatchedTime(t *testing.T) {
	tf.UnitTest(t)

	blockTime := clock.DefaultEpochDuration
	genTime := time.Unix(1234567890, 1234567890%int64(time.Second))
	fc := clock.NewFake(genTime)
	mclock := clock.NewChainClockFromClock(uint64(genTime.Unix()), blockTime, clock.DefaultPropagationDelay, fc)
	validator := consensus.NewDefaultBlockValidator(mclock, nil, nil)

	fc.Advance(blockTime)

	// Passes with correct timestamp
	c := &block.Block{Height: 1, Timestamp: uint64(fc.Now().Unix())}
	require.NoError(t, validator.NotFutureBlock(c))

	// fails with invalid timestamp
	c = &block.Block{Height: 1, Timestamp: uint64(genTime.Unix())}
	err := validator.NotFutureBlock(c)
	if err != nil {
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "wrong epoch")
	}
}

func TestFutureEpoch(t *testing.T) {
	tf.UnitTest(t)

	blockTime := clock.DefaultEpochDuration
	genTime := time.Unix(1234567890, 1234567890%int64(time.Second))
	fc := clock.NewFake(genTime)
	mclock := clock.NewChainClockFromClock(uint64(genTime.Unix()), blockTime, clock.DefaultPropagationDelay, fc)
	validator := consensus.NewDefaultBlockValidator(mclock, nil, nil)

	// Fails in future epoch
	c := &block.Block{Height: 1, Timestamp: uint64(genTime.Add(blockTime).Unix())}
	err := validator.NotFutureBlock(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "block was from the future")
}

func TestBlockValidSyntax(t *testing.T) {
	tf.UnitTest(t)

	blockTime := clock.DefaultEpochDuration
	ts := time.Unix(1234567890, 0)
	mclock := clock.NewFake(ts)
	chainClock := clock.NewChainClockFromClock(uint64(ts.Unix()), blockTime, clock.DefaultPropagationDelay, mclock)
	ctx := context.Background()
	mclock.Advance(blockTime)

	validator := consensus.NewDefaultBlockValidator(chainClock, nil, nil)

	validTs := uint64(mclock.Now().Unix())
	validSt := e.NewCid(types.NewCidForTestGetter()())
	validAd := types.NewForTestGetter()()
	validTi := block.Ticket{VRFProof: []byte{1}}
	// create a valid block
	blk := &block.Block{
		Timestamp: validTs,
		StateRoot: validSt,
		Miner:     validAd,
		Ticket:    validTi,
		Height:    1,

		BlockSig: &crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: []byte{0x3},
		},
	}
	require.NoError(t, validator.ValidateSyntax(ctx, blk))

	// below we will invalidate each part of the block, assert that it fails
	// validation, then revalidate the block

	// invalidate timestamp
	blk.Timestamp = uint64(ts.Add(time.Duration(3) * blockTime).Unix())
	require.Error(t, validator.ValidateSyntax(ctx, blk))
	blk.Timestamp = validTs
	require.NoError(t, validator.ValidateSyntax(ctx, blk))

	// invalidate stateroot
	blk.StateRoot = e.NewCid(cid.Undef)
	require.Error(t, validator.ValidateSyntax(ctx, blk))
	blk.StateRoot = validSt
	require.NoError(t, validator.ValidateSyntax(ctx, blk))

	// invalidate miner address
	blk.Miner = address.Undef
	require.Error(t, validator.ValidateSyntax(ctx, blk))
	blk.Miner = validAd
	require.NoError(t, validator.ValidateSyntax(ctx, blk))

	// invalidate ticket
	blk.Ticket = block.Ticket{}
	require.Error(t, validator.ValidateSyntax(ctx, blk))
	blk.Ticket = validTi
	require.NoError(t, validator.ValidateSyntax(ctx, blk))

}

type fakeMsgSource struct {
	blsMessages  []*types.UnsignedMessage
	secpMessages []*types.SignedMessage
}

func (fms *fakeMsgSource) LoadMetaMessages(ctx context.Context, c cid.Cid) ([]*types.SignedMessage, []*types.UnsignedMessage, error) {
	return fms.secpMessages, fms.blsMessages, nil
}

func (fms *fakeMsgSource) LoadReceipts(context.Context, cid.Cid) ([]types.MessageReceipt, error) {
	return nil, nil
}

type fakeChainState struct {
	actor *types.Actor
	err   error
}

func (fcs *fakeChainState) GetActorAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (*types.Actor, error) {
	return fcs.actor, fcs.err
}

func (fcs *fakeChainState) GetTipSet(block.TipSetKey) (*block.TipSet, error) {
	return &block.TipSet{}, nil
}

func (fcs *fakeChainState) GetTipSetStateRoot(context.Context, block.TipSetKey) (cid.Cid, error) {
	return cid.Undef, nil
}

func (fcs *fakeChainState) StateView(block.TipSetKey, abi.ChainEpoch) (*state.View, error) {
	return nil, nil
}

func (fcs *fakeChainState) GetBlock(context.Context, cid.Cid) (*block.Block, error) {
	return nil, nil
}
