package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p"
	"gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p/p2p/protocol/ping"
	"gx/ipfs/QmNmJZL7FQySMtE2BQuLMuZg2EB2CLEunJJUSVSc9YnnbV/go-libp2p-host"
	ds "gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"
	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"
	"gx/ipfs/QmSFihvoND3eDaAYRCeLgLPt62yCPgMZs1NSZmKFEtJQQw/go-libp2p-floodsub"
	bstore "gx/ipfs/QmTVDM4LCSUMFNQzbDLL9zQwp8usE6QHymFdh3h8vL9v6b/go-ipfs-blockstore"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	nonerouting "gx/ipfs/QmZRcGYvxdauCd7hHnMYLYqcZRaDjv24c7eUNyJojAcdBb/go-ipfs-routing/none"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	bserv "github.com/ipfs/go-ipfs/blockservice"
	exchange "github.com/ipfs/go-ipfs/exchange"
	bitswap "github.com/ipfs/go-ipfs/exchange/bitswap"
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	types "github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
)

var log = logging.Logger("node") // nolint: deadcode

// Node represents a full Filecoin node.
type Node struct {
	Host host.Host

	ChainMgr *core.ChainManager
	MsgPool  *core.MessagePool

	Wallet *wallet.Wallet

	// Mining stuff.
	MiningWorker mining.Worker
	cancelMining context.CancelFunc
	miningDoneWg sync.WaitGroup

	// Network Fields
	PubSub     *floodsub.PubSub
	BlockSub   *floodsub.Subscription
	MessageSub *floodsub.Subscription
	Ping       *ping.PingService
	HelloSvc   *core.Hello

	// Data Storage Fields

	// Datastore is the underlying storage backend.
	Datastore ds.Batching

	// Exchange is the interface for fetching data from other nodes.
	Exchange exchange.Interface

	// Blockservice is a higher level interface for fetching data
	Blockservice bserv.BlockService

	// CborStore is a temporary interface for interacting with IPLD objects.
	CborStore *hamt.CborIpldStore

	// cancelBlockSubscriptionCtx is a handle to cancel the block subscription.
	cancelBlockSubscriptionCtx context.CancelFunc

	// bestBlockCh is a subscription to the best block topic on the chainmgr.
	bestBlockCh chan interface{}
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

// Build instantiates a filecoin Node from the settings specified in the config.
func (nc *Config) Build(ctx context.Context) (*Node, error) {
	host, err := libp2p.New(ctx, nc.Libp2pOpts...)
	if err != nil {
		return nil, err
	}

	// set up pinger
	pinger := ping.NewPingService(host)

	if nc.Datastore == nil {
		nc.Datastore = ds.NewMapDatastore()
	}

	bs := bstore.NewBlockstore(nc.Datastore)

	// no content routing yet...
	routing, _ := nonerouting.ConstructNilRouting(ctx, nil, nil)

	// set up bitswap
	nwork := bsnet.NewFromIpfsHost(host, routing)
	bswap := bitswap.New(ctx, host.ID(), nwork, bs, true)

	bserv := bserv.New(bs, bswap)

	cst := &hamt.CborIpldStore{Blocks: bserv}

	chainMgr := core.NewChainManager(cst)

	msgPool := core.NewMessagePool()

	// Set up but don't start a mining.Worker. It sleeps mineSleepTime
	// to simulate the work of generating proofs.
	blockGenerator := mining.NewBlockGenerator(msgPool, func(ctx context.Context, cid *cid.Cid) (types.StateTree, error) {
		return types.LoadStateTree(ctx, cst, cid)
	}, core.ProcessBlock)
	miningWorker := mining.NewWorkerWithMineAndWork(blockGenerator, mining.Mine, func() { time.Sleep(mineSleepTime) })

	// TODO: load state from disk
	if err := chainMgr.Genesis(ctx, core.InitGenesis); err != nil {
		return nil, err
	}

	// Set up 'hello' handshake service
	hello := core.NewHello(host, chainMgr.GetGenesisCid(), chainMgr.InformNewBlock, chainMgr.GetBestBlock)

	// Set up libp2p pubsub
	fsub, err := floodsub.NewFloodSub(ctx, host)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up floodsub")
	}

	return &Node{
		Blockservice: bserv,
		CborStore:    cst,
		ChainMgr:     chainMgr,
		Datastore:    nc.Datastore,
		Exchange:     bswap,
		HelloSvc:     hello,
		Host:         host,
		MiningWorker: miningWorker,
		MsgPool:      msgPool,
		Ping:         pinger,
		PubSub:       fsub,
		Wallet:       wallet.New(),
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

	// subscribe to message notifications
	msgSub, err := node.PubSub.Subscribe(MessageTopic)
	if err != nil {
		return errors.Wrap(err, "failed to subscribe to message topic")
	}
	node.MessageSub = msgSub

	ctx, cancel := context.WithCancel(context.Background())
	node.cancelBlockSubscriptionCtx = cancel
	go node.handleBlockSubscription(ctx)

	return nil
}

func (node *Node) cancelBlockSubscription() {
	if node.BlockSub != nil {
		node.BlockSub.Cancel()
		node.cancelBlockSubscriptionCtx()
		node.BlockSub = nil
	}
}

// Stop initiates the shutdown of the node.
func (node *Node) Stop() {
	node.cancelBlockSubscription()
	node.ChainMgr.Stop()

	if err := node.Host.Close(); err != nil {
		fmt.Printf("error closing host: %s\n", err)
	}
	fmt.Println("stopping filecoin :(")
}

// How long the node's mining Worker should sleep after it
// generates a new block.
const mineSleepTime = 20 * time.Second

type newBlockFunc func(context.Context, *types.Block)

// StartMining starts the mining worker. The method is separated
// into two pieces so we can capture the output of the worker in
// tests.
// TODO make Start/StopMining safe to call multiple times.
func (node *Node) StartMining(ctx context.Context) {
	node.startMining(ctx, func(ctx context.Context, b *types.Block) {
		if err := node.AddNewBlock(ctx, b); err != nil {
			// Not really an error; a better block could have
			// arrived while mining.
			log.Warningf("Error adding new mined block: %s", err.Error())
		}
	})
}

func (node *Node) startMining(ctx context.Context, newBlock newBlockFunc) {
	miningCtx, cancel := context.WithCancel(ctx)
	node.cancelMining = cancel
	inCh, outCh, workerDoneWg := node.MiningWorker.Start(miningCtx)
	node.bestBlockCh = node.ChainMgr.BestBlockPubSub.Sub(core.BlockTopic)

	// Wire up the mining input.
	go func() {
		inCh <- node.ChainMgr.GetBestBlock()

		for blk := range node.bestBlockCh {
			inCh <- blk.(*types.Block)
		}
		close(inCh)
	}()

	// Wire up the mining output.
	node.miningDoneWg.Add(1)
	go func() {
		// When this goroutine exits wait for the worker to fully exit
		// and then signal our exit on miningDoneWg.
		defer func() {
			workerDoneWg.Wait()
			node.miningDoneWg.Done()
		}()
		for {
			select {
			case <-miningCtx.Done():
				return
			case result := <-outCh:
				if result.Err != nil {
					log.Errorf("Problem mining a block: %s", result.Err.Error())
				} else {
					node.miningDoneWg.Add(1)
					go func() {
						newBlock(miningCtx, result.NewBlock)
						node.miningDoneWg.Done()
					}()
				}
			}
		}
	}()
}

// StopMining stops the node's mining.Worker and waits for it to exit.
func (node *Node) StopMining() {
	node.ChainMgr.BestBlockPubSub.Unsub(node.bestBlockCh)
	node.cancelMining()
	node.miningDoneWg.Wait()
}

// GetSignature fetches the signature for the given method on the appropriate actor.
func (node *Node) GetSignature(ctx context.Context, actorAddr types.Address, method string) (*core.FunctionSignature, error) {
	st, err := types.LoadStateTree(ctx, node.CborStore, node.ChainMgr.GetBestBlock().StateRoot)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load state tree")
	}

	actor, err := st.GetActor(ctx, actorAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get actor")
	}

	executable, err := core.LoadCode(actor.Code)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load actor code")
	}

	export, ok := executable.Exports()[method]
	if !ok {
		return nil, fmt.Errorf("missing export: %s", method)
	}

	return export, nil
}
