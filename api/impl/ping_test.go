package impl

import (
	"context"
	"testing"
	"time"

	pstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/node"
)

func TestPing(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	ctx := context.Background()

	nodes := node.MakeNodesUnstarted(t, 2, false)
	node.StartNodes(t, nodes)
	api0 := New(nodes[0])
	p1 := nodes[1].Host().ID()
	pi1 := pstore.PeerInfo{
		ID:    p1,
		Addrs: nodes[1].Host().Addrs(),
	}

	// connect the nodes
	nodes[0].Host().Connect(ctx, pi1)

	ch, err := api0.Ping().Ping(ctx, p1, 5, 10*time.Millisecond)
	require.NoError(err)

	var out []*api.PingResult
	for p := range ch {
		out = append(out, p)
	}

	// first, plus 6 pings
	require.Len(out, 6)
}
