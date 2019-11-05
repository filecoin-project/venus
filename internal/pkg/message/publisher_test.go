package message_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

func TestDefaultMessagePublisher_Publish(t *testing.T) {
	pool := message.NewPool(config.NewDefaultConfig().Mpool, testhelpers.NewMockMessagePoolValidator())

	ms, _ := types.NewMockSignersAndKeyInfo(2)
	msg := types.NewUnsignedMessage(ms.Addresses[0], ms.Addresses[1], 0, types.ZeroAttoFIL, types.InvalidMethodID, []byte{})
	signed, err := types.NewSignedMessage(*msg, ms)
	require.NoError(t, err)
	msgCid, err := signed.Cid()
	require.NoError(t, err)
	encoded, e := signed.Marshal()
	require.NoError(t, e)

	testCases := []struct {
		name  string
		bcast bool
	}{
		{"Msg added to pool and Publish is called when bcast is true", true},
		{"Msg added to pool and Publish is NOT called when bcast is false", false},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			mnp := message.MockNetworkPublisher{}
			pub := message.NewDefaultPublisher(&mnp, pool)
			assert.NoError(t, pub.Publish(context.Background(), signed, 0, test.bcast))
			smsg, ok := pool.Get(msgCid)
			assert.True(t, ok)
			assert.NotNil(t, smsg)
			if test.bcast {
				assert.Equal(t, encoded, mnp.Data)
			} else {
				assert.Nil(t, mnp.Data)
			}
		})
	}
}
