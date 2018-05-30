package node

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/stretchr/testify/require"
)

// MakeNodesUnstarted creates n new (unstarted) nodes with an InMemoryRepo,
// applies options from the InMemoryRepo and returns a slice of the initialized
// nodes
func MakeNodesUnstarted(t *testing.T, n int, offlineMode bool) []*Node {
	t.Helper()
	var out []*Node
	for i := 0; i < n; i++ {
		r := repo.NewInMemoryRepo()
		err := Init(context.Background(), r)
		require.NoError(t, err)

		if !offlineMode {
			r.Config().Swarm.Address = "/ip4/127.0.0.1/tcp/0"
		}

		opts, err := OptionsFromRepo(r)
		require.NoError(t, err)

		// disables libp2p
		opts = append(opts, func(c *Config) error {
			c.OfflineMode = offlineMode
			return nil
		})
		nd, err := New(context.Background(), opts...)
		require.NoError(t, err)
		out = append(out, nd)
	}

	return out
}

// MakeNodesStarted creates n new (started) nodes with an InMemoryRepo,
// applies options from the InMemoryRepo and returns a slice of the nodes
func MakeNodesStarted(t *testing.T, n int, offlineMode bool) []*Node {
	t.Helper()
	nds := MakeNodesUnstarted(t, n, offlineMode)
	for _, n := range nds {
		require.NoError(t, n.Start())
	}
	return nds
}

// MakeOfflineNode returns a single unstarted offline node.
func MakeOfflineNode(t *testing.T) *Node {
	return MakeNodesUnstarted(t, 1, true)[0]
}
