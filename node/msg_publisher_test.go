package node

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestDefaultMessagePublisher_Publish(t *testing.T) {
	pool := core.NewMessagePool(config.NewDefaultConfig().Mpool, testhelpers.NewMockMessagePoolValidator())

	ms, _ := types.NewMockSignersAndKeyInfo(2)
	msg := types.NewMessage(ms.Addresses[0], ms.Addresses[1], 0, types.ZeroAttoFIL, "", []byte{})
	signed, err := types.NewSignedMessage(*msg, ms, types.ZeroAttoFIL, types.NewGasUnits(0))
	require.NoError(t, err)

	t.Run("Publish is called when bcast is true", func(t *testing.T) {
		mnp := mockNetworkPublisher{}
		pub := newDefaultMessagePublisher(&mnp, "Stuff", pool)
		assert.NoError(t, pub.Publish(context.Background(), signed, 0, true))
		assert.Equal(t, "Stuff", mnp.Topic)
	})

	t.Run("Publish is not called when bcast is false", func(t *testing.T) {
		mnp := mockNetworkPublisher{}
		pub := newDefaultMessagePublisher(&mnp, "Stuff", pool)
		assert.NoError(t, pub.Publish(context.Background(), signed, 0, false))
		assert.Empty(t, mnp.Topic)
	})
}

type mockNetworkPublisher struct {
	Topic string
}

func (mnp *mockNetworkPublisher) Publish(topic string, data []byte) error {
	mnp.Topic = topic
	return nil
}
