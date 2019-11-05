package message_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

var mockSigner, _ = types.NewMockSignersAndKeyInfo(10)
var newSignedMessage = types.NewSignedMessageForTestGetter(mockSigner)

func TestMessagePoolAddRemove(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	pool := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
	msg1 := newSignedMessage()
	msg2 := mustSetNonce(mockSigner, newSignedMessage(), 1)

	c1, err := msg1.Cid()
	assert.NoError(t, err)
	c2, err := msg2.Cid()
	assert.NoError(t, err)

	assert.Len(t, pool.Pending(), 0)
	m, ok := pool.Get(c1)
	assert.Nil(t, m)
	assert.False(t, ok)

	_, err = pool.Add(ctx, msg1, 0)
	assert.NoError(t, err)
	assert.Len(t, pool.Pending(), 1)

	_, err = pool.Add(ctx, msg2, 0)
	assert.NoError(t, err)
	assert.Len(t, pool.Pending(), 2)

	m, ok = pool.Get(c1)
	assert.Equal(t, msg1, m)
	assert.True(t, ok)
	m, ok = pool.Get(c2)
	assert.Equal(t, msg2, m)
	assert.True(t, ok)

	pool.Remove(c1)
	assert.Len(t, pool.Pending(), 1)
	pool.Remove(c2)
	assert.Len(t, pool.Pending(), 0)
}

func TestMessagePoolValidate(t *testing.T) {
	tf.UnitTest(t)

	t.Run("message pool rejects messages after it reaches its limit", func(t *testing.T) {
		// pull the default size from the default config value
		mpoolCfg := config.NewDefaultConfig().Mpool
		maxMessagePoolSize := mpoolCfg.MaxPoolSize
		ctx := context.Background()
		pool := message.NewPool(mpoolCfg, th.NewMockMessagePoolValidator())

		smsgs := types.NewSignedMsgs(maxMessagePoolSize+1, mockSigner)
		for _, smsg := range smsgs[:maxMessagePoolSize] {
			_, err := pool.Add(ctx, smsg, 0)
			require.NoError(t, err)
		}

		assert.Len(t, pool.Pending(), int(maxMessagePoolSize))

		// attempt to add one more
		_, err := pool.Add(ctx, smsgs[maxMessagePoolSize], 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "message pool is full")

		assert.Len(t, pool.Pending(), int(maxMessagePoolSize))
	})

	t.Run("validates no two messages are added with same nonce", func(t *testing.T) {
		ctx := context.Background()
		pool := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		smsg1 := newSignedMessage()
		_, err := pool.Add(ctx, smsg1, 0)
		require.NoError(t, err)

		smsg2 := mustSetNonce(mockSigner, newSignedMessage(), smsg1.Message.CallSeqNum)
		_, err = pool.Add(ctx, smsg2, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "message with same actor and nonce")
	})

	t.Run("validates using supplied validator", func(t *testing.T) {
		ctx := context.Background()
		validator := th.NewMockMessagePoolValidator()
		validator.Valid = false
		pool := message.NewPool(config.NewDefaultConfig().Mpool, validator)

		smsg1 := mustSetNonce(mockSigner, newSignedMessage(), 0)
		_, err := pool.Add(ctx, smsg1, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mock validation error")
	})
}

func TestMessagePoolDedup(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	pool := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
	msg1 := newSignedMessage()

	assert.Len(t, pool.Pending(), 0)
	_, err := pool.Add(ctx, msg1, 0)
	assert.NoError(t, err)
	assert.Len(t, pool.Pending(), 1)

	_, err = pool.Add(ctx, msg1, 0)
	assert.NoError(t, err)
	assert.Len(t, pool.Pending(), 1)
}

func TestMessagePoolAsync(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	count := uint(400)
	mpoolCfg := config.NewDefaultConfig().Mpool
	mpoolCfg.MaxPoolSize = count
	msgs := types.NewSignedMsgs(count, mockSigner)

	pool := message.NewPool(mpoolCfg, th.NewMockMessagePoolValidator())
	var wg sync.WaitGroup

	for i := uint(0); i < 4; i++ {
		wg.Add(1)
		go func(i uint) {
			for j := uint(0); j < count/4; j++ {
				_, err := pool.Add(ctx, msgs[j+(count/4)*i], 0)
				assert.NoError(t, err)
			}
			wg.Done()
		}(i)
	}

	wg.Wait()
	assert.Len(t, pool.Pending(), int(count))
}

func TestLargestNonce(t *testing.T) {
	tf.UnitTest(t)

	t.Run("No matches", func(t *testing.T) {
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewSignedMsgs(2, mockSigner)
		reqAdd(t, p, 0, m[0], m[1])

		_, found := p.LargestNonce(address.NewForTestGetter()())
		assert.False(t, found)
	})

	t.Run("Match, largest is zero", func(t *testing.T) {
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewMsgsWithAddrs(1, mockSigner.Addresses)
		m[0].CallSeqNum = 0

		sm, err := types.SignMsgs(mockSigner, m)
		require.NoError(t, err)

		reqAdd(t, p, 0, sm...)

		largest, found := p.LargestNonce(m[0].From)
		assert.True(t, found)
		assert.Equal(t, uint64(0), largest)
	})

	t.Run("Match", func(t *testing.T) {
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewMsgsWithAddrs(3, mockSigner.Addresses)
		m[1].CallSeqNum = 1
		m[2].CallSeqNum = 2
		m[2].From = m[1].From

		sm, err := types.SignMsgs(mockSigner, m)
		require.NoError(t, err)

		reqAdd(t, p, 0, sm...)

		largest, found := p.LargestNonce(m[2].From)
		assert.True(t, found)
		assert.Equal(t, uint64(2), largest)
	})
}

func mustSetNonce(signer types.Signer, message *types.SignedMessage, nonce types.Uint64) *types.SignedMessage {
	return mustResignMessage(signer, message, func(m *types.UnsignedMessage) {
		m.CallSeqNum = nonce
	})
}

func mustResignMessage(signer types.Signer, message *types.SignedMessage, f func(*types.UnsignedMessage)) *types.SignedMessage {
	var msg types.UnsignedMessage
	msg = message.Message
	f(&msg)
	smg, err := signMessage(signer, msg)
	if err != nil {
		panic("Error signing message")
	}
	return smg
}

func signMessage(signer types.Signer, message types.UnsignedMessage) (*types.SignedMessage, error) {
	return types.NewSignedMessage(message, signer)
}

func reqAdd(t *testing.T, p *message.Pool, height uint64, msgs ...*types.SignedMessage) {
	ctx := context.Background()
	for _, m := range msgs {
		_, err := p.Add(ctx, m, height)
		require.NoError(t, err)
	}
}
