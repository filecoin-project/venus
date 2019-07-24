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
	msgCid, err := signed.Cid()
	require.NoError(t, err)

	testCases := []struct {
		name     string
		bcast    bool
		expected string
	}{
		{"Msg added to pool and Publish is called when bcast is true", true, "Stuff"},
		{"Msg added to pool and Publish is NOT called when bcast is false", false, ""},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			mnp := mockNetworkPublisher{}
			pub := newDefaultMessagePublisher(&mnp, "Stuff", pool)
			assert.NoError(t, pub.Publish(context.Background(), signed, 0, test.bcast))
			smsg, ok := pool.Get(msgCid)
			assert.True(t, ok)
			assert.NotNil(t, smsg)
			assert.Equal(t, test.expected, mnp.Topic)
		})
	}
}

type mockNetworkPublisher struct {
	Topic string
}

func (mnp *mockNetworkPublisher) Publish(topic string, data []byte) error {
	mnp.Topic = topic
	return nil
}
