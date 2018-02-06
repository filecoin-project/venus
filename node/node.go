package node

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/types"

	"gx/ipfs/QmSwABWvsucRwH7XVDbTE3aJgiVdtJUfeWjY8oejB4RmAA/go-hamt-ipld"
	"gx/ipfs/QmT68EgyFbnSs5rbHkNkFZQwjdHfqrJiGr3R6rwcwnzYLc/go-libp2p"
	"gx/ipfs/QmfCtHMCd9xFvehvHeVxtKVXJTMVTuHhyPRVHEXetn87vL/go-libp2p-host"

	"github.com/filecoin-project/go-filecoin/chain"
)

// Node represents a full Filecoin node.
type Node struct {
	Block *types.Block
	Host  host.Host

	ChainMgr *chain.ChainManager

	CborStore *hamt.CborIpldStore
}

// Config is a helper to aid in the construction of a filecoin node.
type Config struct {
	Libp2pOpts []libp2p.Option
}

// ConfigOpt is a configuration option for a filecoin node.
type ConfigOpt func(*Config) error

// Libp2pOptions returns a node config option that sets up the libp2p node
func Libp2pOptions(opts ...libp2p.Option) ConfigOpt {
	return func(nc *Config) error {
		nc.Libp2pOpts = opts
		return nil
	}
}

// New creates a new node.
func New(ctx context.Context, opts ...ConfigOpt) (*Node, error) {
	n := &Config{}
	for _, o := range opts {
		if err := o(n); err != nil {
			return nil, err
		}
	}

	return n.Build(ctx)
}

// Build instantiates a filecoin Node from the settings specified in the
// config.
func (nc *Config) Build(ctx context.Context) (*Node, error) {
	host, err := libp2p.New(ctx, nc.Libp2pOpts...)
	if err != nil {
		return nil, err
	}
	cst := hamt.NewCborStore()

	chainMgr := chain.NewChainManager(cst)

	// TODO: load state from disk
	if err := chainMgr.SetBestBlock(ctx, chain.GenesisBlock); err != nil {
		return nil, err
	}

	return &Node{
		CborStore: cst,
		Host:      host,
		ChainMgr:  chainMgr,
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
