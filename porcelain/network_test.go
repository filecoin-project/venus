package porcelain_test

import (
	"context"
	"testing"
	"time"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/porcelain"
)

type pingTestPlumbing struct{}

func (p *pingTestPlumbing) NetworkGetPeerID() peer.ID {
	// Use any peer id except nil so the IDs don't match
	id, _ := peer.IDB58Decode("QmWbMozPyW6Ecagtxq7SXBXXLY5BNdP1GwHB2WoZCKMvcb")
	return id
}

func (p *pingTestPlumbing) NetworkPing(ctx context.Context, pid peer.ID) (<-chan time.Duration, error) {
	duration, err := time.ParseDuration("1s")
	if err != nil {
		return nil, err
	}
	out := make(chan time.Duration)
	go func() {
		for i := 0; i < 3; i++ {
			out <- duration
		}
	}()
	return out, nil
}

func TestNetworkPingWithCount(t *testing.T) {
	t.Parallel()

	t.Run("returns times given by network ping", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		ctx := context.Background()
		plumbing := &pingTestPlumbing{}
		pid := peer.ID("")
		duration, err := time.ParseDuration("1s")
		require.NoError(err)

		results, err := porcelain.NetworkPingWithCount(ctx, plumbing, pid, 3, duration)
		require.NoError(err)

		expectedFirstDuration, err := time.ParseDuration("0s")
		require.NoError(err)
		expectedFirstResult := &porcelain.PingResult{
			Time:    expectedFirstDuration,
			Text:    "PING <peer.ID >",
			Success: false,
		}

		firstResult := <-results
		assert.Equal(expectedFirstResult, firstResult)

		expectedDuration, err := time.ParseDuration("1s")
		require.NoError(err)
		expectedResult := &porcelain.PingResult{
			Time:    expectedDuration,
			Text:    "",
			Success: true,
		}

		for result := range results {
			assert.Equal(expectedResult, result)
		}
	})
}
