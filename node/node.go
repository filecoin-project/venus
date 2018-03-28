package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p"
	"gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p/p2p/protocol/ping"
	"gx/ipfs/QmNmJZL7FQySMtE2BQuLMuZg2EB2CLEunJJUSVSc9YnnbV/go-libp2p-host"
	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"
	"gx/ipfs/QmSFihvoND3eDaAYRCeLgLPt62yCPgMZs1NSZmKFEtJQQw/go-libp2p-floodsub"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	nonerouting "gx/ipfs/QmXtoXbu9ReyV6Q4kDQ5CF9wXQNDY1PdHc4HhfxRR5AHB3/go-ipfs-routing/none"
	bstore "gx/ipfs/QmaG4DZ4JaqEfvPWt5nPPgoTzhc1tr1T3f4Nu9Jpdm8ymY/go-ipfs-blockstore"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	bserv "github.com/ipfs/go-ipfs/blockservice"
	exchange "github.com/ipfs/go-ipfs/exchange"
	bitswap "github.com/ipfs/go-ipfs/exchange/bitswap"
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"

	"github.com/filecoin-project/go-filecoin/core"
	lookup "github.com/filecoin-project/go-filecoin/lookup"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/repo"
	types "github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
)

var log = logging.Logger("node") // nolint: deadcode

var (
	// ErrNoMethod is returned when processing a message that does not have a method.
	ErrNoMethod = errors.New("no method in message")
)

// Node represents a full Filecoin node.
type Node struct {
	Host host.Host

	ChainMgr *core.ChainManager
	// BestBlockCh is a subscription to the best block topic on the chainmgr.
	BestBlockCh chan interface{}
	// BestBlockHandled is a hook for tests because pubsub notifications
	// arrive async. It's called after handling a new best block.
	BestBlockHandled func()
	MsgPool          *core.MessagePool

	Wallet *wallet.Wallet

	// Mining stuff.
	MiningWorker mining.Worker
	mining       struct {
		sync.Mutex
		isMining bool
	}
	miningInCh         chan<- mining.Input
	miningCtx          context.Context
	cancelMining       context.CancelFunc
	miningDoneWg       *sync.WaitGroup
	AddNewlyMinedBlock newBlockFunc

	StorageClient *StorageClient
	StorageMarket *StorageMarket

	// Network Fields
	PubSub     *floodsub.PubSub
	BlockSub   *floodsub.Subscription
	MessageSub *floodsub.Subscription
	Ping       *ping.PingService
	HelloSvc   *core.Hello

	// Data Storage Fields

	// Repo is the repo this node was created with
	// it contains all persistent artifacts of the filecoin node
	Repo repo.Repo

	// Exchange is the interface for fetching data from other nodes.
	Exchange exchange.Interface

	// Blockservice is a higher level interface for fetching data
	Blockservice bserv.BlockService

	// CborStore is a temporary interface for interacting with IPLD objects.
	CborStore *hamt.CborIpldStore

	// A lookup engine for mapping on-chain address to peerIds
	Lookup *lookup.LookupEngine

	// cancelBlockSubscriptionCtx is a handle to cancel the block subscription.
	cancelBlockSubscriptionCtx context.CancelFunc
}

// Config is a helper to aid in the construction of a filecoin node.
type Config struct {
	Libp2pOpts []libp2p.Option

	Repo repo.Repo
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

	if nc.Repo == nil {
		// TODO: maybe allow for not passing a repo?
		return nil, fmt.Errorf("must pass a repo option to the node build process")
	}

	bs := bstore.NewBlockstore(nc.Repo.Datastore())

	// no content routing yet...
	routing, _ := nonerouting.ConstructNilRouting(ctx, nil, nil)

	// set up bitswap
	nwork := bsnet.NewFromIpfsHost(host, routing)
	bswap := bitswap.New(ctx, host.ID(), nwork, bs, true)

	bserv := bserv.New(bs, bswap)

	cst := &hamt.CborIpldStore{Blocks: bserv}

	chainMgr := core.NewChainManager(nc.Repo.Datastore(), cst)

	msgPool := core.NewMessagePool()

	// Set up but don't start a mining.Worker. It sleeps mineSleepTime
	// to simulate the work of generating proofs.
	blockGenerator := mining.NewBlockGenerator(msgPool, func(ctx context.Context, cid *cid.Cid) (types.StateTree, error) {
		return types.LoadStateTree(ctx, cst, cid)
	}, core.ProcessBlock)
	miningWorker := mining.NewWorkerWithMineAndWork(blockGenerator, mining.Mine, func() { time.Sleep(mineSleepTime) })

	// Set up libp2p pubsub
	fsub, err := floodsub.NewFloodSub(ctx, host)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up floodsub")
	}

	fcWallet := wallet.New()
	le, err := lookup.NewLookupEngine(fsub, fcWallet, host.ID())
	if err != nil {
		return nil, errors.Wrap(err, "failed to setup lookup engine")
	}

	return &Node{
		Blockservice: bserv,
		CborStore:    cst,
		ChainMgr:     chainMgr,
		Exchange:     bswap,
		Host:         host,
		MiningWorker: miningWorker,
		MsgPool:      msgPool,
		Ping:         pinger,
		PubSub:       fsub,
		Repo:         nc.Repo,
		Wallet:       fcWallet,
		Lookup:       le,
	}, nil
}

// Start boots up the node.
func (node *Node) Start() error {

	if err := node.ChainMgr.Load(); err != nil {
		return err
	}

	// Start up 'hello' handshake service
	node.HelloSvc = core.NewHello(node.Host, node.ChainMgr.GetGenesisCid(), node.ChainMgr.InformNewBlock, node.ChainMgr.GetBestBlock)

	node.StorageClient = NewStorageClient(node)
	node.StorageMarket = NewStorageMarket(node)

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
	go node.handleBlockSubscription(ctx, blkSub)

	// Set up mining.Worker. The node won't feed blocks to the worker
	// until node.StartMining() is called.
	node.miningCtx, node.cancelMining = context.WithCancel(context.Background())
	inCh, outCh, doneWg := node.MiningWorker.Start(node.miningCtx)
	node.miningInCh = inCh
	node.miningDoneWg = doneWg
	node.AddNewlyMinedBlock = node.addNewlyMinedBlock
	node.miningDoneWg.Add(1)
	go node.handleNewMiningOutput(outCh)

	node.BestBlockHandled = func() {}
	node.BestBlockCh = node.ChainMgr.BestBlockPubSub.Sub(core.BlockTopic)
	go node.handleNewBestBlock(ctx, node.ChainMgr.GetBestBlock())

	return nil
}

func (node *Node) setIsMining(isMining bool) {
	node.mining.Lock()
	defer node.mining.Unlock()
	node.mining.isMining = isMining
}

func (node *Node) isMining() bool {
	node.mining.Lock()
	defer node.mining.Unlock()
	return node.mining.isMining
}

// TODO this always mines to the 0th address in the wallet.
// We need to enable setting which address.
func (node *Node) getRewardAddress() (types.Address, error) {
	addrs := node.Wallet.GetAddresses()
	if len(addrs) == 0 {
		return types.Address{}, errors.New("No addresses in wallet")
	}
	return addrs[0], nil
}

func (node *Node) handleNewMiningOutput(miningOutCh <-chan mining.Output) {
	defer func() {
		node.miningDoneWg.Done()
	}()
	for {
		select {
		case <-node.miningCtx.Done():
			return
		case output, ok := <-miningOutCh:
			if !ok {
				return
			}
			if output.Err != nil {
				log.Errorf("Problem mining a block: %s", output.Err.Error())
			} else {
				node.miningDoneWg.Add(1)
				go func() {
					if node.isMining() {
						node.AddNewlyMinedBlock(node.miningCtx, output.NewBlock)
					}
					node.miningDoneWg.Done()
				}()
			}
		}
	}

}

func (node *Node) handleNewBestBlock(ctx context.Context, head *types.Block) {
	for blk := range node.BestBlockCh {
		newHead := blk.(*types.Block)
		if err := core.UpdateMessagePool(ctx, node.MsgPool, node.CborStore, head, newHead); err != nil {
			panic(err)
		}
		head = newHead
		if node.isMining() {
			rewardAddress, err := node.getRewardAddress()
			if err != nil {
				log.Error("No mining reward address, mining should not have started!")
				continue
			}
			node.miningDoneWg.Add(1)
			go func() {
				defer func() { node.miningDoneWg.Done() }()
				select {
				case <-node.miningCtx.Done():
					return
				case node.miningInCh <- mining.NewInput(context.Background(), head, rewardAddress):
				}
			}()
		}
		node.BestBlockHandled()
	}
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
	node.ChainMgr.BestBlockPubSub.Unsub(node.BestBlockCh)
	if node.cancelMining != nil {
		node.cancelMining()
	}
	if node.miningDoneWg != nil {
		node.miningDoneWg.Wait()
	}
	if node.miningInCh != nil {
		close(node.miningInCh)
	}
	node.cancelBlockSubscription()
	node.ChainMgr.Stop()

	if err := node.Host.Close(); err != nil {
		fmt.Printf("error closing host: %s\n", err)
	}

	if err := node.Repo.Close(); err != nil {
		fmt.Printf("error closing repo: %s\n", err)
	}

	fmt.Println("stopping filecoin :(")
}

// How long the node's mining Worker should sleep after it
// generates a new block.
const mineSleepTime = 20 * time.Second

type newBlockFunc func(context.Context, *types.Block)

func (node *Node) addNewlyMinedBlock(ctx context.Context, b *types.Block) {
	if err := node.AddNewBlock(ctx, b); err != nil {
		// Not really an error; a better block could have
		// arrived while mining.
		log.Warningf("Error adding new mined block: %s", err.Error())
	}
}

// StartMining causes the node to start feeding blocks to the mining worker.
func (node *Node) StartMining() error {
	rewardAddress, err := node.getRewardAddress()
	if err != nil {
		return err
	}
	node.setIsMining(true)
	node.miningDoneWg.Add(1)
	go func() {
		defer func() { node.miningDoneWg.Done() }()
		select {
		case <-node.miningCtx.Done():
			return
		case node.miningInCh <- mining.NewInput(context.Background(), node.ChainMgr.GetBestBlock(), rewardAddress):
		}
	}()
	return nil
}

// StopMining stops mining on new blocks.
func (node *Node) StopMining() {
	// TODO should probably also keep track of and cancel last mining.Input.Ctx.
	node.setIsMining(false)
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

	if method == "" {
		// this is allowed if it is a transfer only case
		return nil, ErrNoMethod
	}

	export, ok := executable.Exports()[method]
	if !ok {
		return nil, fmt.Errorf("missing export: %s", method)
	}

	return export, nil
}
