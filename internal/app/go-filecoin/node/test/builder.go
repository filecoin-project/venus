package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/node"
	"github.com/filecoin-project/venus/internal/pkg/config"
	"github.com/filecoin-project/venus/internal/pkg/genesis"
	"github.com/filecoin-project/venus/internal/pkg/repo"
	gengen "github.com/filecoin-project/venus/tools/gengen/util"
)

// NodeBuilder creates and configures Filecoin nodes for in-process testing.
// This is intended to replace use of GenNode and the various other node construction entry points
// that end up there.
// Note that (August 2019) there are two things called "config": the configuration read in from
// file to the config.Config structure, and node.Config which is really just some dependency
// injection. This builder avoids exposing the latter directly.
type NodeBuilder struct {
	// Initialisation function for the genesis block and state.
	gif genesis.InitFunc
	// Options to the repo initialisation.
	initOpts []node.InitOpt
	// Mutations to be applied to node config after initialisation.
	configMutations []node.ConfigOpt
	// Mutations to be applied to the node builder config before building.
	builderOpts []node.BuilderOpt

	tb testing.TB
}

// NewNodeBuilder creates a new node builder.
func NewNodeBuilder(tb testing.TB) *NodeBuilder {
	return &NodeBuilder{
		gif:      gengen.MakeGenesisFunc(gengen.NetworkName("gfctest")),
		initOpts: []node.InitOpt{},
		configMutations: []node.ConfigOpt{
			// Default configurations that make sense for integration tests.
			// The can be overridden by subsequent `withConfigChanges`.
			node.ConfigOpt(func(c *config.Config) {
				// Bind only locally, defer port selection until binding.
				c.API.Address = "/ip4/127.0.0.1/tcp/0"
				c.Swarm.Address = "/ip4/127.0.0.1/tcp/0"
			}),
			node.ConfigOpt(func(c *config.Config) {
				c.Bootstrap.MinPeerThreshold = 0
			}),
		},
		builderOpts: []node.BuilderOpt{},
		tb:          tb,
	}
}

// WithGenesisInit sets the built nodes' genesis function.
func (b *NodeBuilder) WithGenesisInit(gif genesis.InitFunc) *NodeBuilder {
	b.gif = gif
	return b
}

// WithInitOpt adds one or more options to repo initialisation.
func (b *NodeBuilder) WithInitOpt(opts ...node.InitOpt) *NodeBuilder {
	b.initOpts = append(b.initOpts, opts...)
	return b
}

// WithBuilderOpt adds one or more node building options to node creation.
func (b *NodeBuilder) WithBuilderOpt(opts ...node.BuilderOpt) *NodeBuilder {
	b.builderOpts = append(b.builderOpts, opts...)
	return b
}

// WithConfig adds a configuration mutation function to be invoked after repo initialisation.
func (b *NodeBuilder) WithConfig(cm node.ConfigOpt) *NodeBuilder {
	b.configMutations = append(b.configMutations, cm)
	return b
}

// Build creates a node as specified by this builder.
// This many be invoked multiple times to create many nodes.
func (b *NodeBuilder) Build(ctx context.Context) *node.Node {
	// Initialise repo.
	repo := repo.NewInMemoryRepo()

	for _, opt := range b.configMutations {
		opt(repo.Config())
	}
	b.requireNoError(node.Init(ctx, repo, b.gif, b.initOpts...))

	// Initialize the node.
	repoConfigOpts, err := node.OptionsFromRepo(repo)
	b.requireNoError(err)

	nd, err := node.New(ctx, append(repoConfigOpts, b.builderOpts...)...)
	b.requireNoError(err)
	return nd
}

// BuildAndStart build a node and starts it.
func (b *NodeBuilder) BuildAndStart(ctx context.Context) *node.Node {
	n := b.Build(ctx)
	err := n.Start(ctx)
	b.requireNoError(err)
	return n
}

// BuildAndStartAPI is a convenience function composing BuildAndStart with
// RunNodeAPI
func (b *NodeBuilder) BuildAndStartAPI(ctx context.Context) (*node.Node, *Client, func()) {
	n := b.BuildAndStart(ctx)
	c, apiDone := RunNodeAPI(ctx, n, b.tb)
	done := func() {
		apiDone()
		n.Stop(ctx)
	}
	return n, c, done
}

func (b *NodeBuilder) requireNoError(err error) {
	b.tb.Helper()
	require.NoError(b.tb, err)
}

// BuildMany builds numNodes nodes with the builder's configuration.
func (b *NodeBuilder) BuildMany(ctx context.Context, numNodes int) []*node.Node {
	var out []*node.Node
	for i := 0; i < numNodes; i++ {
		nd := b.Build(ctx)
		out = append(out, nd)
	}

	return out
}
