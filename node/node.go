package node

import (
	"context"
	"fmt"
	"sync"

	bserv "gx/ipfs/QmNUCLv5fmUBuAcwbkt58NQvMcJgd5FPCYV2yNCXq4Wnd6/go-ipfs/blockservice"
	"gx/ipfs/QmNUCLv5fmUBuAcwbkt58NQvMcJgd5FPCYV2yNCXq4Wnd6/go-ipfs/exchange/bitswap"
	bsnet "gx/ipfs/QmNUCLv5fmUBuAcwbkt58NQvMcJgd5FPCYV2yNCXq4Wnd6/go-ipfs/exchange/bitswap/network"
	libp2p "gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p"
	"gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p/p2p/protocol/ping"
	host "gx/ipfs/QmNmJZL7FQySMtE2BQuLMuZg2EB2CLEunJJUSVSc9YnnbV/go-libp2p-host"
	logging "gx/ipfs/QmQCqiR5F3NeJRr7LuWq8i8FgtT65ypZw5v9V6Es6nwFBD/go-log"
	floodsub "gx/ipfs/QmSFihvoND3eDaAYRCeLgLPt62yCPgMZs1NSZmKFEtJQQw/go-libp2p-floodsub"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	nonerouting "gx/ipfs/QmXtoXbu9ReyV6Q4kDQ5CF9wXQNDY1PdHc4HhfxRR5AHB3/go-ipfs-routing/none"
	bstore "gx/ipfs/QmaG4DZ4JaqEfvPWt5nPPgoTzhc1tr1T3f4Nu9Jpdm8ymY/go-ipfs-blockstore"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	exchange "gx/ipfs/QmdcAXgEHUueP4A7b5hjabKn2EooeHgMreMvFC249dGCgc/go-ipfs-exchange-interface"
	hamt "gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/filnet"
	"github.com/filecoin-project/go-filecoin/lookup"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
)

var log = logging.Logger("node") // nolint: deadcode

var (
	// ErrNoMethod is returned when processing a message that does not have a method.
	ErrNoMethod = errors.New("no method in message")
	// ErrNoRepo is returned when the configs repo is nil
	ErrNoRepo = errors.New("must pass a repo option to the node build process")
	// ErrNoRewardAddress is returned when the node is not configured to have reward address.
	ErrNoRewardAddress = errors.New("no reward address configured")
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

	// Storage Market Interfaces
	StorageClient *StorageClient
	StorageMarket *StorageMarket

	// Network Fields
	PubSub       *floodsub.PubSub
	BlockSub     *floodsub.Subscription
	MessageSub   *floodsub.Subscription
	Ping         *ping.PingService
	HelloSvc     *core.Hello
	Bootstrapper *filnet.Bootstrapper

	// Data Storage Fields

	// Repo is the repo this node was created with
	// it contains all persistent artifacts of the filecoin node
	Repo          repo.Repo
	SectorBuilder *SectorBuilder

	// Exchange is the interface for fetching data from other nodes.
	Exchange exchange.Interface

	// Blockservice is a higher level interface for fetching data
	Blockservice bserv.BlockService

	// CborStore is a temporary interface for interacting with IPLD objects.
	CborStore *hamt.CborIpldStore

	// A lookup engine for mapping on-chain address to peerIds
	Lookup *lookup.LookupEngine

	// cancelSubscriptionsCtx is a handle to cancel the block and message subscriptions.
	cancelSubscriptionsCtx context.CancelFunc

	// OfflineMode, when true, disables libp2p
	OfflineMode bool

	rewardAddress types.Address
}

// Config is a helper to aid in the construction of a filecoin node.
type Config struct {
	Libp2pOpts    []libp2p.Option
	Repo          repo.Repo
	OfflineMode   bool
	RewardAddress types.Address
}

// ConfigOpt is a configuration option for a filecoin node.
type ConfigOpt func(*Config) error

// Libp2pOptions returns a node config option that sets up the libp2p node
func Libp2pOptions(opts ...libp2p.Option) ConfigOpt {
	return func(nc *Config) error {
		// Quietly having your options overridden leads to hair loss
		if len(nc.Libp2pOpts) > 0 {
			panic("Libp2pOptions can only be called once")
		}
		nc.Libp2pOpts = opts
		return nil
	}
}

// RewardAddress returns a node config option that sets the reward address on the node.
func RewardAddress(addr types.Address) ConfigOpt {
	return func(nc *Config) error {
		nc.RewardAddress = addr
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
	var host host.Host

	if !nc.OfflineMode {
		h, err := libp2p.New(ctx, nc.Libp2pOpts...)
		if err != nil {
			return nil, err
		}

		host = h
	} else {
		host = noopLibP2PHost{}
	}

	// set up pinger
	pinger := ping.NewPingService(host)

	if nc.Repo == nil {
		nc.Repo = repo.NewInMemoryRepo()
	}

	bs := bstore.NewBlockstore(nc.Repo.Datastore())

	// no content routing yet...
	routing, _ := nonerouting.ConstructNilRouting(ctx, nil, nil)

	// set up bitswap
	nwork := bsnet.NewFromIpfsHost(host, routing)
	bswap := bitswap.New(ctx, nwork, bs)
	bserv := bserv.New(bs, bswap)

	cst := &hamt.CborIpldStore{Blocks: bserv}

	chainMgr := core.NewChainManager(nc.Repo.Datastore(), cst)

	msgPool := core.NewMessagePool()

	// Set up but don't start a mining.Worker. It sleeps mineSleepTime
	// to simulate the work of generating proofs.
	blockGenerator := mining.NewBlockGenerator(msgPool, func(ctx context.Context, cid *cid.Cid) (state.Tree, error) {
		return state.LoadStateTree(ctx, cst, cid, builtin.Actors)
	}, mining.ApplyMessages)
	miningWorker := mining.NewWorker(blockGenerator)

	// Set up libp2p pubsub
	fsub, err := floodsub.NewFloodSub(ctx, host)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up floodsub")
	}
	backend, err := wallet.NewDSBackend(nc.Repo.WalletDatastore())
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up wallet backend")
	}
	fcWallet := wallet.New(backend)

	le, err := lookup.NewLookupEngine(fsub, fcWallet, host.ID())
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up lookup engine")
	}

	nd := &Node{
		Blockservice:  bserv,
		CborStore:     cst,
		ChainMgr:      chainMgr,
		Exchange:      bswap,
		Host:          host,
		Lookup:        le,
		MiningWorker:  miningWorker,
		MsgPool:       msgPool,
		OfflineMode:   nc.OfflineMode,
		Ping:          pinger,
		PubSub:        fsub,
		Repo:          nc.Repo,
		Wallet:        fcWallet,
		rewardAddress: nc.RewardAddress,
	}

	dirs := nd.Repo.(SectorDirs)
	var sb *SectorBuilder
	sb, err = NewSectorBuilder(nd, sectorSize, dirs)
	if err != nil {
		return nil, err
	}
	// TODO: initialize SectorBuilder from metadata.
	nd.SectorBuilder = sb

	// Bootstrapping network peers.
	ba := nd.Repo.Config().Bootstrap.Addresses
	bpi, err := filnet.PeerAddrsToPeerInfos(ba)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't parse bootstrap addresses [%s]", ba)
	}
	nd.Bootstrapper = filnet.NewBootstrapper(bpi, nd.Host, nd.Host.Network())

	return nd, nil
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
	node.cancelSubscriptionsCtx = cancel

	go node.handleSubscription(ctx, node.processBlock, "processBlock", node.BlockSub, "BlockSub")
	go node.handleSubscription(ctx, node.processMessage, "processMessage", node.MessageSub, "MessageSub")

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

	if !node.OfflineMode {
		node.Bootstrapper.Start(context.Background())
	}

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

// RewardAddress returns the configured reward address for this node.
func (node *Node) RewardAddress() types.Address {
	return node.rewardAddress
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
	// TODO(EC): The current mining code is driven by the promotion of a new best
	// block. This should probably change for EC: we want to tell the mining worker
	// about *all* arriving tipsets at the current height. So the way we currently
	// feed best blocks should probably be changed to feed tipsets at the current
	// height.
	for blk := range node.BestBlockCh {
		newHead := blk.(*types.Block)
		// TODO(EC): When a new best block is promoted we remove messages in it from the
		// message pool (and add them back in if we have a re-org). The way we update the
		// message pool should probably change for EC -- tipsets have multiple blocks, not
		// a single block. See also note in BlockGenerator that suggests we shouldn't be
		// clearing the pool out there; I think I agree, we should instead do something more
		// like garbage collection periodically.
		if err := core.UpdateMessagePool(ctx, node.MsgPool, node.CborStore, head, newHead); err != nil {
			panic(err)
		}
		head = newHead
		if node.isMining() {
			if node.rewardAddress == (types.Address{}) {
				log.Error("No mining reward address, mining should not have started!")
				continue
			}
			node.miningDoneWg.Add(1)
			go func() {
				defer func() { node.miningDoneWg.Done() }()
				// TODO(EC): for now wrap the new best block (head of the chain) in a
				// TipSet. TipSet arrival hasn't yet been plumbed through here.
				tipSets := []core.TipSet{{head.Cid().String(): head}}
				select {
				case <-node.miningCtx.Done():
					return
				case node.miningInCh <- mining.NewInput(context.Background(), tipSets, node.rewardAddress):
				}
			}()
		}
		node.BestBlockHandled()
	}
}

func (node *Node) cancelSubscriptions() {
	if node.BlockSub != nil || node.MessageSub != nil {
		node.cancelSubscriptionsCtx()
	}

	if node.BlockSub != nil {
		node.BlockSub.Cancel()
		node.BlockSub = nil
	}

	if node.MessageSub != nil {
		node.MessageSub.Cancel()
		node.MessageSub = nil
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
	node.cancelSubscriptions()
	node.ChainMgr.Stop()

	if err := node.Host.Close(); err != nil {
		fmt.Printf("error closing host: %s\n", err)
	}

	if err := node.Repo.Close(); err != nil {
		fmt.Printf("error closing repo: %s\n", err)
	}

	node.Bootstrapper.Stop()

	fmt.Println("stopping filecoin :(")
}

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
	if node.rewardAddress == (types.Address{}) {
		return ErrNoRewardAddress
	}
	node.setIsMining(true)
	node.miningDoneWg.Add(1)
	go func() {
		defer func() { node.miningDoneWg.Done() }()
		// TODO(EC): Here is where we kick mining off when we start off. Will
		// need to change to pass in best tipsets, of which there can be multiple.
		bb := node.ChainMgr.GetBestBlock()
		tipSets := []core.TipSet{{bb.Cid().String(): bb}}
		select {
		case <-node.miningCtx.Done():
			return
		case node.miningInCh <- mining.NewInput(context.Background(), tipSets, node.rewardAddress):
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
func (node *Node) GetSignature(ctx context.Context, actorAddr types.Address, method string) (*exec.FunctionSignature, error) {
	st, err := state.LoadStateTree(ctx, node.CborStore, node.ChainMgr.GetBestBlock().StateRoot, builtin.Actors)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load state tree")
	}

	actor, err := st.GetActor(ctx, actorAddr)
	if err != nil || actor.Code == nil {
		return nil, errors.Wrap(err, "failed to get actor")
	}

	executable, err := st.GetBuiltinActorCode(actor.Code)
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

// NextNonce returns the next nonce for the given address. It checks
// the actor's memory and also scans the message pool for any pending
// messages.
func NextNonce(ctx context.Context, node *Node, address types.Address) (uint64, error) {
	bb := node.ChainMgr.GetBestBlock()
	st, err := state.LoadStateTree(ctx, node.CborStore, bb.StateRoot, builtin.Actors)
	if err != nil {
		return 0, err
	}
	nonce, err := core.NextNonce(ctx, st, node.MsgPool, address)
	if err != nil {
		return 0, err
	}
	return nonce, nil
}

// NewMessageWithNextNonce returns a new types.Message whose
// nonce is set to our best guess at the next appropriate value
// (see NextNonce).
func NewMessageWithNextNonce(ctx context.Context, node *Node, from, to types.Address, value *types.TokenAmount, method string, params []byte) (*types.Message, error) {
	nonce, err := NextNonce(ctx, node, from)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get next nonce")
	}
	return types.NewMessage(from, to, nonce, value, method, params), nil
}

// NewAddress creates a new account address on the default wallet backend.
func (node *Node) NewAddress() (types.Address, error) {
	backends := node.Wallet.Backends(wallet.DSBackendType)
	if len(backends) == 0 {
		return types.Address{}, fmt.Errorf("missing default ds backend")
	}

	backend := (backends[0]).(*wallet.DSBackend)
	return backend.NewAddress()
}

// QueryMessage sends a read-only message to an actor to retrieve some of its current (best block) state.
func (node *Node) QueryMessage(msg *types.Message) ([]byte, uint8, error) {
	ctx := context.Background()
	bb := node.ChainMgr.GetBestBlock()
	st, err := state.LoadStateTree(ctx, node.CborStore, bb.StateRoot, builtin.Actors)
	if err != nil {
		return nil, 1, err
	}

	return core.ApplyQueryMessage(ctx, st, msg, types.NewBlockHeight(bb.Height))
}

// CreateMiner creates a new miner actor for the given account and returns its address.
// It will wait for the the actor to appear on-chain and add it's address to mining.minerAddresses in the config.
// TODO: This should live in a MinerAPI or some such. It's here until we have a proper API layer.
func (node *Node) CreateMiner(ctx context.Context, accountAddr types.Address, pledge types.BytesAmount, collateral types.TokenAmount) (*types.Address, error) {
	// TODO: pull public key from wallet
	params, err := abi.ToEncodedValues(&pledge, []byte{})
	if err != nil {
		return nil, err
	}

	msg, err := NewMessageWithNextNonce(ctx, node, accountAddr, address.StorageMarketAddress, &collateral, "createMiner", params)
	if err != nil {
		return nil, err
	}

	if err := node.AddNewMessage(ctx, msg); err != nil {
		return nil, err
	}

	msgCid, err := msg.Cid()
	if err != nil {
		return nil, err
	}

	var minerAddress types.Address
	err = node.ChainMgr.WaitForMessage(ctx, msgCid, func(blk *types.Block, msg *types.Message,
		receipt *types.MessageReceipt) error {
		minerAddress, err = types.NewAddressFromBytes(receipt.ReturnValue())
		return err
	})
	if err != nil {
		return nil, err
	}

	err = node.saveMinerAddressToConfig(minerAddress)
	return &minerAddress, err
}

func (node *Node) saveMinerAddressToConfig(addr types.Address) error {
	r := node.Repo
	newConfig := r.Config()
	newConfig.Mining.MinerAddresses = append(newConfig.Mining.MinerAddresses, addr)

	return r.ReplaceConfig(newConfig)
}
