package node

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/stretchr/testify/require"
)

// NewInMemoryNode creates a new node with an InMemoryRepo, applies options
// from the InMemoryRepo and returns the initialized node
func NewInMemoryNode(t *testing.T, ctx context.Context) *Node { // nolint: golint
	r := repo.NewInMemoryRepo()
	require.NoError(t, Init(ctx, r))

	opts, err := OptionsFromRepo(r)
	require.NoError(t, err)

	node, err := New(ctx, opts...)
	require.NoError(t, err)

	return node
}
