package node

import (
	"context"
	"fmt"

	"gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p"
	"gx/ipfs/QmNmJZL7FQySMtE2BQuLMuZg2EB2CLEunJJUSVSc9YnnbV/go-libp2p-host"
	ds "gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"
	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"
	"gx/ipfs/QmSFihvoND3eDaAYRCeLgLPt62yCPgMZs1NSZmKFEtJQQw/go-libp2p-floodsub"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmdBXcN47jVwKLwSyN9e9xYVZ7WcAWgQ5N4cmNw7nzWq2q/go-hamt-ipld"

	bstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	exchange "github.com/ipfs/go-ipfs/exchange"
	bitswap "github.com/ipfs/go-ipfs/exchange/bitswap"
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	nonerouting "github.com/ipfs/go-ipfs/routing/none"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/types"
)

var log = logging.Logger("node")

// Node represents a full Filecoin node.
type Node struct {
	Host host.Host

	ChainMgr *chain.ChainManager

	Wallet *types.Wallet

	// Network Fields
	PubSub   *floodsub.PubSub
	BlockSub *floodsub.Subscription

	// Data Storage Fields
	Datastore ds.Batching
	Exchange  exchange.Interface
	CborStore *hamt.CborIpldStore
}

// Config is a helper to aid in the construction of a filecoin node.
type Config struct {
	Libp2pOpts []libp2p.Option

	Datastore ds.Batching
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

	if nc.Datastore == nil {
		nc.Datastore = ds.NewMapDatastore()
	}

	bs := bstore.NewBlockstore(nc.Datastore)

	// no content routing yet...
	routing, _ := nonerouting.ConstructNilRouting(nil, nil, nil)

	// set up bitswap
	nwork := bsnet.NewFromIpfsHost(host, routing)
	bswap := bitswap.New(ctx, host.ID(), nwork, bs, true)

	bserv := bserv.New(bs, bswap)

	cst := &hamt.CborIpldStore{bserv}

	chainMgr := chain.NewChainManager(cst)

	// TODO: load state from disk
	if err := chainMgr.SetBestBlock(ctx, chain.GenesisBlock); err != nil {
		return nil, err
	}

	// make sure we have the genesis block stored
	_, err = cst.Put(ctx, chain.GenesisBlock)
	if err != nil {
		return nil, errors.Wrap(err, "failed to add genesis block to local datastore")
	}

	// Set up libp2p pubsub
	fsub, err := floodsub.NewFloodSub(ctx, host)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up floodsub")
	}

	return &Node{
		CborStore: cst,
		Host:      host,
		ChainMgr:  chainMgr,
		PubSub:    fsub,
		Datastore: nc.Datastore,
		Exchange:  bswap,
		Wallet:    types.NewWallet(),
	}, nil
}

// Start boots up the node.
func (node *Node) Start() error {
	// subscribe to block notifications
	blkSub, err := node.PubSub.Subscribe(BlocksTopic)
	if err != nil {
		return errors.Wrap(err, "failed to subscribe to blocks topic")
	}
	node.BlockSub = blkSub

	go node.handleBlockSubscription()

	return nil
}

// Stop initiates the shutdown of the node.
func (node *Node) Stop() {
	if node.BlockSub != nil {
		node.BlockSub.Cancel()
		node.BlockSub = nil
	}

	if err := node.Host.Close(); err != nil {
		fmt.Printf("error closing host: %s\n", err)
	}
	fmt.Println("stopping filecoin :(")
}
