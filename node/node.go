package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gx/ipfs/QmPMtD39NN63AEUNghk1LFQcTLcCmYL8MtRzdv8BRUsC4Z/go-libp2p-host"
	"gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	libp2ppeer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"
	"gx/ipfs/QmT5K5mHn2KUyCDBntKoojQJAJftNzutxzpYR33w8JdN6M/go-libp2p-floodsub"
	bserv "gx/ipfs/QmTfTKeBhTLjSjxXQsjkF2b1DfZmYEMnknGE2y2gX57C6v/go-blockservice"
	"gx/ipfs/QmTwzvuH2eYPJLdp3sL4ZsdSwCcqzdwc1Vk9ssVbk25EA2/go-bitswap"
	bsnet "gx/ipfs/QmTwzvuH2eYPJLdp3sL4ZsdSwCcqzdwc1Vk9ssVbk25EA2/go-bitswap/network"
	"gx/ipfs/QmVM6VuGaWcAaYjxG2om6XxMmpP3Rt9rw4nbMXVNYAPLhS/go-libp2p"
	"gx/ipfs/QmVM6VuGaWcAaYjxG2om6XxMmpP3Rt9rw4nbMXVNYAPLhS/go-libp2p/p2p/protocol/ping"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmWw71Mz9PXKgYG8ZfTYN7Ax2Zm48Eurbne3wC2y7CKmLz/go-ipfs-exchange-interface"
	nonerouting "gx/ipfs/QmYYXrfYh14XcN5jhmK31HhdAG85HjHAg5czk3Eb9cGML4/go-ipfs-routing/none"
	bstore "gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/filnet"
	"github.com/filecoin-project/go-filecoin/lookup"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	vmErrors "github.com/filecoin-project/go-filecoin/vm/errors"
	"github.com/filecoin-project/go-filecoin/wallet"
)

var log = logging.Logger("node") // nolint: deadcode

var (
	// ErrNoMethod is returned when processing a message that does not have a method.
	ErrNoMethod = errors.New("no method in message")
	// ErrNoRepo is returned when the configs repo is nil
	ErrNoRepo = errors.New("must pass a repo option to the node build process")
	// ErrNoMinerAddress is returned when the node is not configured to have any miner addresses.
	ErrNoMinerAddress = errors.New("no miner addresses configured")
	// ErrNoDefaultMessageFromAddress is returned when the node's wallet is not configured to have a default address and the wallet contains more than one address.
	ErrNoDefaultMessageFromAddress = errors.New("could not produce a from-address for message sending")
)

// Node represents a full Filecoin node.
type Node struct {
	Host host.Host

	ChainMgr *core.ChainManager
	// HeavyTipSetCh is a subscription to the heaviest tipset topic on the chainmgr.
	HeaviestTipSetCh chan interface{}
	// HeavyTipSetHandled is a hook for tests because pubsub notifications
	// arrive async. It's called after handling a new heaviest tipset.
	HeaviestTipSetHandled func()
	MsgPool               *core.MessagePool

	Wallet *wallet.Wallet

	// Mining stuff.
	MiningScheduler mining.Scheduler
	mining          struct {
		sync.Mutex
		isMining bool
	}
	miningInCh         chan<- mining.Input
	miningCtx          context.Context
	cancelMining       context.CancelFunc
	miningDoneWg       *sync.WaitGroup
	AddNewlyMinedBlock newBlockFunc
	blockTime          time.Duration

	// Storage Market Interfaces
	StorageClient      *StorageClient
	StorageBroker      *StorageBroker
	StorageMinerClient *StorageMinerClient
	StorageMiner       *StorageMiner

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
	Repo repo.Repo

	// SectorBuilders are used by the miners to fill and seal sectors
	SectorBuilders map[address.Address]*SectorBuilder

	// Exchange is the interface for fetching data from other nodes.
	Exchange exchange.Interface

	// Blockstore is the un-networked blocks interface
	Blockstore bstore.Blockstore

	// Blockservice is a higher level interface for fetching data
	Blockservice bserv.BlockService

	// CborStore is a temporary interface for interacting with IPLD objects.
	CborStore *hamt.CborIpldStore

	// A lookup service for mapping on-chain miner address to libp2p identity.
	Lookup lookup.PeerLookupService

	// cancelSubscriptionsCtx is a handle to cancel the block and message subscriptions.
	cancelSubscriptionsCtx context.CancelFunc

	// OfflineMode, when true, disables libp2p
	OfflineMode bool

	// mockMineMode, when true mocks mining and validation logic for tests.
	// TODO: this is a TEMPORARY workaround
	mockMineMode bool
}

// Config is a helper to aid in the construction of a filecoin node.
type Config struct {
	Libp2pOpts   []libp2p.Option
	Repo         repo.Repo
	OfflineMode  bool
	MockMineMode bool // TODO: this is a TEMPORARY workaround
	BlockTime    time.Duration
}

// ConfigOpt is a configuration option for a filecoin node.
type ConfigOpt func(*Config) error

// OfflineMode enables or disables offline mode.
func OfflineMode(offlineMode bool) ConfigOpt {
	return func(c *Config) error {
		c.OfflineMode = offlineMode
		return nil
	}
}

// BlockTime sets the blockTime.
func BlockTime(blockTime time.Duration) ConfigOpt {
	return func(c *Config) error {
		c.BlockTime = blockTime
		return nil
	}
}

// MockMineMode enables or disable mocked mining.
func MockMineMode(mockMineMode bool) ConfigOpt {
	return func(c *Config) error {
		c.MockMineMode = mockMineMode
		return nil
	}
}

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
	routing, _ := nonerouting.ConstructNilRouting(ctx, nil, nil, nil)

	// set up bitswap
	nwork := bsnet.NewFromIpfsHost(host, routing)
	bswap := bitswap.New(ctx, nwork, bs)
	bserv := bserv.New(bs, bswap)

	cst := &hamt.CborIpldStore{Blocks: bserv}

	chainMgr := core.NewChainManager(nc.Repo.Datastore(), bs, cst)
	if nc.MockMineMode {
		chainMgr.PwrTableView = &core.TestView{}
	}

	msgPool := core.NewMessagePool()

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

	nd := &Node{
		Blockservice:   bserv,
		Blockstore:     bs,
		CborStore:      cst,
		ChainMgr:       chainMgr,
		Exchange:       bswap,
		Host:           host,
		MsgPool:        msgPool,
		OfflineMode:    nc.OfflineMode,
		Ping:           pinger,
		PubSub:         fsub,
		Repo:           nc.Repo,
		SectorBuilders: make(map[address.Address]*SectorBuilder),
		Wallet:         fcWallet,
		mockMineMode:   nc.MockMineMode,
		blockTime:      nc.BlockTime,
	}

	// Bootstrapping network peers.
	ba := nd.Repo.Config().Bootstrap.Addresses
	bpi, err := filnet.PeerAddrsToPeerInfos(ba)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't parse bootstrap addresses [%s]", ba)
	}
	nd.Bootstrapper = filnet.NewBootstrapper(bpi, nd.Host, nd.Host.Network())

	// On-chain lookup service
	nd.Lookup = lookup.NewChainLookupService(chainMgr, nd.DefaultSenderAddress)

	return nd, nil
}

// Start boots up the node.
func (node *Node) Start(ctx context.Context) error {
	if err := node.ChainMgr.Load(); err != nil {
		return err
	}

	// Start up 'hello' handshake service
	node.HelloSvc = core.NewHello(node.Host, node.ChainMgr.GetGenesisCid(), node.ChainMgr.InformNewTipSet, node.ChainMgr.GetHeaviestTipSet)

	node.StorageClient = NewStorageClient(node)
	node.StorageBroker = NewStorageBroker(node)
	node.StorageMiner = NewStorageMiner(node)
	node.StorageMinerClient = NewStorageMinerClient(node)

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

	cctx, cancel := context.WithCancel(context.Background())
	node.cancelSubscriptionsCtx = cancel

	go node.handleSubscription(cctx, node.processBlock, "processBlock", node.BlockSub, "BlockSub")
	go node.handleSubscription(cctx, node.processMessage, "processMessage", node.MessageSub, "MessageSub")

	node.HeaviestTipSetHandled = func() {}
	node.HeaviestTipSetCh = node.ChainMgr.HeaviestTipSetPubSub.Sub(core.HeaviestTipSetTopic)
	go node.handleNewHeaviestTipSet(cctx, node.ChainMgr.GetHeaviestTipSet())

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

func (node *Node) handleNewHeaviestTipSet(ctx context.Context, head core.TipSet) {
	currentBestCancel := func() {}
	for ts := range node.HeaviestTipSetCh {
		var currentBestCtx context.Context
		newHead := ts.(core.TipSet)
		if len(newHead) == 0 {
			log.Error("TipSet of size 0 published on HeaviestTipSetCh:")
			log.Error("ignoring and waiting for a new Heaviest TipSet.")
		}

		// When a new best TipSet is promoted we remove messages in it from the
		// message pool (and add them back in if we have a re-org).
		if err := core.UpdateMessagePool(ctx, node.MsgPool, node.CborStore, head, newHead); err != nil {
			panic(err)
		}
		head = newHead
		if node.isMining() {
			// The miningInCh should accept tipsets with strictly increasing
			// weight, so cancel the previous input.
			currentBestCancel()
			currentBestCtx, currentBestCancel = context.WithCancel(context.Background())
			node.miningDoneWg.Add(1)
			go func() {
				defer func() { node.miningDoneWg.Done() }()
				select {
				case <-node.miningCtx.Done(): // mining is done
					return
				case <-currentBestCtx.Done(): // this input is no longer the heaviest
					return
				case node.miningInCh <- mining.NewInput(head):
				}
			}()
		}
		node.HeaviestTipSetHandled()
	}
	currentBestCancel() // keep the linter happy
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
func (node *Node) Stop(ctx context.Context) {
	node.ChainMgr.HeaviestTipSetPubSub.Unsub(node.HeaviestTipSetCh)
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

// MiningAddress returns the address of the mining actor mining on behalf of
// the node.
func (node *Node) MiningAddress() (address.Address, error) {
	// TODO: this is a temporary workaround to permit nodes to mine without setup.
	if node.mockMineMode {
		return node.DefaultSenderAddress()
	}
	r := node.Repo
	newConfig := r.Config()
	if len(newConfig.Mining.MinerAddresses) == 0 {
		return address.Address{}, ErrNoMinerAddress
	}
	// TODO: mining start should include a flag to specify a specific
	// mining addr.  For now default to the first created.
	return newConfig.Mining.MinerAddresses[0], nil
}

// MiningTimes returns the configured time it takes to mine a block, and also
// the mining delay duration, which is currently a fixed fraction of block time.
// Note this is mocked behavior, in production this time is determined by how
// long it takes to generate PoSTs.
func (node *Node) MiningTimes() (time.Duration, time.Duration) {
	mineDelay := node.blockTime / mining.MineDelayConversionFactor
	return node.blockTime, mineDelay
}

// StartMining causes the node to start feeding blocks to the mining worker and initializes
// a SectorBuilder for each mining address.
func (node *Node) StartMining(ctx context.Context) error {
	miningAddress, err := node.MiningAddress()
	if err != nil {
		return err
	}
	blockTime, mineDelay := node.MiningTimes()

	if node.MiningScheduler == nil {
		getStateTree := func(ctx context.Context, ts core.TipSet) (state.Tree, error) {
			return node.ChainMgr.State(ctx, ts.ToSlice())
		}
		worker := mining.NewDefaultWorker(node.MsgPool, getStateTree, node.ChainMgr.Weight, core.ApplyMessages, node.ChainMgr.PwrTableView, node.Blockstore, node.CborStore, miningAddress, blockTime)
		node.MiningScheduler = mining.NewScheduler(worker, mineDelay)
		node.miningCtx, node.cancelMining = context.WithCancel(context.Background())
		inCh, outCh, doneWg := node.MiningScheduler.Start(node.miningCtx)
		node.miningInCh = inCh
		node.miningDoneWg = doneWg
		node.AddNewlyMinedBlock = node.addNewlyMinedBlock
		node.miningDoneWg.Add(1)
		go node.handleNewMiningOutput(outCh)
	}

	if err := node.initSectorBuilder(miningAddress); err != nil {
		return errors.Wrap(err, "failed to initialize sector builder")
	}

	node.setIsMining(true)
	node.miningDoneWg.Add(1)
	go func() {
		defer func() { node.miningDoneWg.Done() }()
		hts := node.ChainMgr.GetHeaviestTipSet()
		select {
		case <-node.miningCtx.Done():
			return
		case node.miningInCh <- mining.NewInput(hts):
		}
	}()

	return nil
}

func (node *Node) initSectorBuilder(minerAddr address.Address) error {
	dirs := node.Repo.(SectorDirs)

	sb, err := InitSectorBuilder(node, minerAddr, sectorSize, dirs)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to initialize sector builder for miner %s", minerAddr.String()))
	}

	node.SectorBuilders[minerAddr] = sb

	return nil
}

// StopMining stops mining on new blocks.
func (node *Node) StopMining(ctx context.Context) {
	node.setIsMining(false)
}

// GetSignature fetches the signature for the given method on the appropriate actor.
func (node *Node) GetSignature(ctx context.Context, actorAddr address.Address, method string) (_ *exec.FunctionSignature, err error) {
	ctx = log.Start(ctx, "Node.GetSignature")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	st, err := node.ChainMgr.State(ctx, node.ChainMgr.GetHeaviestTipSet().ToSlice())
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
func NextNonce(ctx context.Context, node *Node, address address.Address) (nonce uint64, err error) {
	ctx = log.Start(ctx, "Node.NextNonce")
	defer func() {
		log.SetTag(ctx, "nonce", nonce)
		log.FinishWithErr(ctx, err)
	}()

	st, err := node.ChainMgr.State(ctx, node.ChainMgr.GetHeaviestTipSet().ToSlice())
	if err != nil {
		return 0, err
	}
	nonce, err = core.NextNonce(ctx, st, node.MsgPool, address)
	if err != nil {
		return 0, err
	}
	return nonce, nil
}

// NewMessageWithNextNonce returns a new types.Message whose
// nonce is set to our best guess at the next appropriate value
// (see NextNonce).
func NewMessageWithNextNonce(ctx context.Context, node *Node, from, to address.Address, value *types.AttoFIL, method string, params []byte) (_ *types.Message, err error) {
	ctx = log.Start(ctx, "Node.NewMessageWithNextNonce")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	nonce, err := NextNonce(ctx, node, from)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get next nonce")
	}
	return types.NewMessage(from, to, nonce, value, method, params), nil
}

// NewAddress creates a new account address on the default wallet backend.
func (node *Node) NewAddress() (address.Address, error) {
	backends := node.Wallet.Backends(wallet.DSBackendType)
	if len(backends) == 0 {
		return address.Address{}, fmt.Errorf("missing default ds backend")
	}

	backend := (backends[0]).(*wallet.DSBackend)
	return backend.NewAddress()
}

// CallQueryMethod calls a method on an actor using the state of the heaviest
// tipset. It doesn't make any changes to the state/blockchain. It is useful
// for interrogating actor state. The caller address is optional; if not
// provided, an address will be chosen from the node's wallet.
func (node *Node) CallQueryMethod(ctx context.Context, to address.Address, method string, args []byte, optFrom *address.Address) (_ [][]byte, _ uint8, err error) {
	ctx = log.Start(ctx, "Node.CallQueryMethod")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	bts := node.ChainMgr.GetHeaviestTipSet()
	st, err := node.ChainMgr.State(ctx, bts.ToSlice())
	if err != nil {
		return nil, 1, errors.Wrap(err, "failed to retrieve state")
	}
	h, err := bts.Height()
	if err != nil {
		return nil, 1, errors.Wrap(err, "getting base tipset height")
	}

	fromAddr, err := node.DefaultSenderAddress()
	if err != nil {
		return nil, 1, errors.Wrap(err, "failed to retrieve default sender address")
	}

	if optFrom != nil {
		fromAddr = *optFrom
	}

	vms := vm.NewStorageMap(node.Blockstore)
	return core.CallQueryMethod(ctx, st, vms, to, method, args, fromAddr, types.NewBlockHeight(h))
}

// CreateMiner creates a new miner actor for the given account and returns its address.
// It will wait for the the actor to appear on-chain and add its address to mining.minerAddresses in the config.
// TODO: This should live in a MinerAPI or some such. It's here until we have a proper API layer.
func (node *Node) CreateMiner(ctx context.Context, accountAddr address.Address, pledge types.BytesAmount, pid libp2ppeer.ID, collateral types.AttoFIL) (_ *address.Address, err error) {
	ctx = log.Start(ctx, "Node.CreateMiner")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	// TODO: make this more streamlined in the wallet
	backend, err := node.Wallet.Find(accountAddr)
	if err != nil {
		return nil, err
	}
	info, err := backend.GetKeyInfo(accountAddr)
	if err != nil {
		return nil, err
	}
	pubkey, err := info.PublicKey()
	if err != nil {
		return nil, err
	}
	params, err := abi.ToEncodedValues(&pledge, pubkey, pid)
	if err != nil {
		return nil, err
	}

	msg, err := NewMessageWithNextNonce(ctx, node, accountAddr, address.StorageMarketAddress, &collateral, "createMiner", params)
	if err != nil {
		return nil, err
	}

	smsg, err := types.NewSignedMessage(*msg, node.Wallet)
	if err != nil {
		return nil, err
	}

	if err := node.AddNewMessage(ctx, smsg); err != nil {
		return nil, err
	}

	smsgCid, err := smsg.Cid()
	if err != nil {
		return nil, err
	}

	var minerAddress address.Address
	err = node.ChainMgr.WaitForMessage(ctx, smsgCid, func(blk *types.Block, smsg *types.SignedMessage,
		receipt *types.MessageReceipt) error {
		if receipt.ExitCode != uint8(0) {
			return vmErrors.VMExitCodeToError(receipt.ExitCode, storagemarket.Errors)
		}
		minerAddress, err = address.NewFromBytes(receipt.Return[0])
		return err
	})
	if err != nil {
		return nil, err
	}

	err = node.saveMinerAddressToConfig(minerAddress)

	// TODO: if the node is mining, should we now create a sector builder
	// for this miner?

	return &minerAddress, err
}

func (node *Node) saveMinerAddressToConfig(addr address.Address) error {
	r := node.Repo
	newConfig := r.Config()
	newConfig.Mining.MinerAddresses = append(newConfig.Mining.MinerAddresses, addr)

	return r.ReplaceConfig(newConfig)
}

// DefaultSenderAddress produces a default address from which to send messages.
func (node *Node) DefaultSenderAddress() (address.Address, error) {
	ret, err := node.defaultWalletAddress()
	if err != nil || ret != (address.Address{}) {
		return ret, err
	}

	if len(node.Wallet.Addresses()) > 0 {
		// TODO: this works for now, but is likely not a great solution.
		// Need to figure out what better behaviour to define in regards
		// to default addresses.
		addr := node.Wallet.Addresses()[0]

		newConfig := node.Repo.Config()
		newConfig.Wallet.DefaultAddress = addr

		if err := node.Repo.ReplaceConfig(newConfig); err != nil {
			return address.Address{}, err
		}

		return addr, nil
	}

	return address.Address{}, ErrNoDefaultMessageFromAddress
}

func (node *Node) defaultWalletAddress() (address.Address, error) {
	addr, err := node.Repo.Config().Get("wallet.defaultAddress")
	if err != nil {
		return address.Address{}, err
	}
	return addr.(address.Address), nil
}
