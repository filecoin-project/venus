package impl

import (
	"context"
	"testing"
	"time"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	pstore "gx/ipfs/QmRhFARzTHcFh8wUxwN5KvyTGq73FLC65EfFAhz8Ng7aGb/go-libp2p-peerstore"

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
