package node

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/types"

	"gx/ipfs/QmT68EgyFbnSs5rbHkNkFZQwjdHfqrJiGr3R6rwcwnzYLc/go-libp2p"
	"gx/ipfs/QmfCtHMCd9xFvehvHeVxtKVXJTMVTuHhyPRVHEXetn87vL/go-libp2p-host"
)

// Node represents a full Filecoin node.
type Node struct {
	Host  host.Host
	Block *types.Block
}

// NodeConfig is a helper to aid in the construction of a filecoin node.
type NodeConfig struct {
	Libp2pOpts []libp2p.Option
}

// NodeOpt is a configuration option for a filecoin node.
type NodeConfigOpt func(*NodeConfig) error

// Libp2pOptions returns a node config option that sets up the libp2p node
func Libp2pOptions(opts ...libp2p.Option) NodeConfigOpt {
	return func(nc *NodeConfig) error {
		nc.Libp2pOpts = opts
		return nil
	}
}

// New creates a new node.
func New(ctx context.Context, opts ...NodeConfigOpt) (*Node, error) {
	n := &NodeConfig{}
	for _, o := range opts {
		if err := o(n); err != nil {
			return nil, err
		}
	}

	return n.Build(ctx)
}

// Build instantiates a filecoin Node from the settings specified in the
// config.
func (nc *NodeConfig) Build(ctx context.Context) (*Node, error) {
	host, err := libp2p.New(ctx, nc.Libp2pOpts...)
	if err != nil {
		return nil, err
	}

	return &Node{
		Host: host,
	}, nil
}

// Start boots up the node.
func (node *Node) Start() error {
	return nil
}

// Stop initiates the shutdown of the node.
func (node *Node) Stop() {
	if err := node.Host.Close(); err != nil {
		fmt.Printf("error closing host: %s\n", err)
	}
	fmt.Println("stopping filecoin :(")
}
