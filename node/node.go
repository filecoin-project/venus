package node

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sync"
	"time"

	ps "github.com/cskr/pubsub"
	"github.com/filecoin-project/go-filecoin/vm"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/clock"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/metrics"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/net"
	"github.com/filecoin-project/go-filecoin/net/pubsub"
	"github.com/filecoin-project/go-filecoin/paths"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/protocol/block"
	"github.com/filecoin-project/go-filecoin/protocol/hello"
	"github.com/filecoin-project/go-filecoin/protocol/retrieval"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/sampling"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/util/moresync"
	"github.com/filecoin-project/go-filecoin/version"
	vmerr "github.com/filecoin-project/go-filecoin/vm/errors"
	"github.com/filecoin-project/go-filecoin/wallet"
)

var log = logging.Logger("node") // nolint: deadcode

var (
	// ErrNoMinerAddress is returned when the node is not configured to have any miner addresses.
	ErrNoMinerAddress = errors.New("no miner addresses configured")
)

type pubSubHandler func(ctx context.Context, msg pubsub.Message) error

type nodeChainReader interface {
	GenesisCid() cid.Cid
	GetHead() types.TipSetKey
	GetTipSet(types.TipSetKey) (types.TipSet, error)
	GetTipSetState(ctx context.Context, tsKey types.TipSetKey) (state.Tree, error)
	HeadEvents() *ps.PubSub
	Load(context.Context) error
	Stop()
}

type nodeChainSyncer interface {
	HandleNewTipSet(ctx context.Context, ci *types.ChainInfo, trusted bool) error
}

// storageFaultSlasher is the interface for needed FaultSlasher functionality
type storageFaultSlasher interface {
	OnNewHeaviestTipSet(context.Context, types.TipSet) error
}

// Node represents a full Filecoin node.
type Node struct {
	host     host.Host
	PeerHost host.Host

	Consensus    consensus.Protocol
	ChainReader  nodeChainReader
	MessageStore *chain.MessageStore
	Syncer       nodeChainSyncer
	PowerTable   consensus.PowerTableView
	NetworkName  string
	VersionTable version.ProtocolVersionTable

	BlockMiningAPI *block.MiningAPI
	PorcelainAPI   *porcelain.API
	RetrievalAPI   *retrieval.API
	StorageAPI     *storage.API

	// HeavyTipSetCh is a subscription to the heaviest tipset topic on the chain.
	// https://github.com/filecoin-project/go-filecoin/issues/2309
	HeaviestTipSetCh chan interface{}
	// cancelChainSync cancels the context for chain sync subscriptions and handlers.
	cancelChainSync context.CancelFunc

	// Incoming messages for block mining.
	Inbox *core.Inbox
	// Messages sent and not yet mined.
	Outbox *core.Outbox

	Wallet *wallet.Wallet

	// Mining stuff.
	AddNewlyMinedBlock newBlockFunc
	// cancelMining cancels the context for block production and sector commitments.
	cancelMining    context.CancelFunc
	MiningWorker    mining.Worker
	MiningScheduler mining.Scheduler
	mining          struct {
		sync.Mutex
		isMining bool
	}
	miningDoneWg *sync.WaitGroup

	// Storage Market Interfaces
	StorageMiner *storage.Miner

	StorageFaultSlasher storageFaultSlasher

	// Retrieval Interfaces
	RetrievalMiner *retrieval.Miner

	// Network Fields
	BlockSub     pubsub.Subscription
	MessageSub   pubsub.Subscription
	HelloSvc     *hello.Handler
	Bootstrapper *net.Bootstrapper

	// Data Storage Fields

	// Repo is the repo this node was created with
	// it contains all persistent artifacts of the filecoin node
	Repo repo.Repo

	// SectorBuilder is used by the miner to fill and seal sectors.
	sectorBuilder sectorbuilder.SectorBuilder

	// PeerTracker maintains a list of peers good for fetching.
	PeerTracker *net.PeerTracker

	// Fetcher is the interface for fetching data from nodes.
	Fetcher net.Fetcher

	// Exchange is the interface for fetching data from other nodes.
	Exchange exchange.Interface

	// Blockstore is the un-networked blocks interface
	Blockstore bstore.Blockstore

	// Blockservice is a higher level interface for fetching data
	blockservice bserv.BlockService

	// CborStore is a temporary interface for interacting with IPLD objects.
	cborStore *hamt.CborIpldStore

	// OfflineMode, when true, disables libp2p
	OfflineMode bool

	// Router is a router from IPFS
	Router routing.Routing

	// ChainSynced is a latch that releases when a nodes chain reaches a caught-up state.
	// It serves as a barrier to be released when the initial chain sync has completed.
	// Services which depend on a more-or-less synced chain can wait for this before starting up.
	ChainSynced *moresync.Latch

	// Clock is a clock used by the node for time.
	Clock clock.Clock
}

type blankValidator struct{}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

// readGenesisCid is a helper function that queries the provided datastore for
// an entry with the genesisKey cid, returning if found.
func readGenesisCid(ds datastore.Datastore) (cid.Cid, error) {
	bb, err := ds.Get(chain.GenesisKey)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to read genesisKey")
	}

	var c cid.Cid
	err = json.Unmarshal(bb, &c)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to cast genesisCid")
	}
	return c, nil
}

// Start boots up the node.
func (node *Node) Start(ctx context.Context) error {
	if err := metrics.RegisterPrometheusEndpoint(node.Repo.Config().Observability.Metrics); err != nil {
		return errors.Wrap(err, "failed to setup metrics")
	}

	if err := metrics.RegisterJaeger(node.host.ID().Pretty(), node.Repo.Config().Observability.Tracing); err != nil {
		return errors.Wrap(err, "failed to setup tracing")
	}

	var err error
	if err = node.ChainReader.Load(ctx); err != nil {
		return err
	}

	// Only set these up if there is a miner configured.
	if _, err := node.MiningAddress(); err == nil {
		if err := node.setupMining(ctx); err != nil {
			log.Errorf("setup mining failed: %v", err)
			return err
		}
	}

	// TODO: defer establishing these API endpoints until the chain is synced when the commands
	//   can handle their absence: https://github.com/filecoin-project/go-filecoin/issues/3137
	err = node.setupProtocols()
	if err != nil {
		return errors.Wrap(err, "failed to set up protocols:")
	}
	node.RetrievalMiner = retrieval.NewMiner(node)

	var syncCtx context.Context
	syncCtx, node.cancelChainSync = context.WithCancel(context.Background())

	// Wire up propagation of new chain heads from the chain store to other components.
	head, err := node.PorcelainAPI.ChainHead()
	if err != nil {
		return errors.Wrap(err, "failed to get chain head")
	}
	go node.handleNewChainHeads(syncCtx, head)

	if !node.OfflineMode {
		// Start bootstrapper.
		node.Bootstrapper.Start(context.Background())

		// Register peer tracker disconnect function with network.
		net.TrackerRegisterDisconnect(node.host.Network(), node.PeerTracker)

		// Start up 'hello' handshake service
		helloCallback := func(ci *types.ChainInfo) {
			node.PeerTracker.Track(ci)
			// TODO Implement principled trusting of ChainInfo's
			// to address in #2674
			trusted := true
			err := node.Syncer.HandleNewTipSet(context.Background(), ci, trusted)
			if err != nil {
				log.Infof("error handling tipset from hello %s: %s", ci, err)
				return
			}
			// For now, consider the initial bootstrap done after the syncer has (synchronously)
			// processed the chain up to the head reported by the first peer to respond to hello.
			// This is an interim sequence until a secure network bootstrap is implemented:
			// https://github.com/filecoin-project/go-filecoin/issues/2674.
			// For now, we trust that the first node to respond will be a configured bootstrap node
			// and that we trust that node to inform us of the chain head.
			// TODO: when the syncer rejects too-far-ahead blocks received over pubsub, don't consider
			// sync done until it's caught up enough that it will accept blocks from pubsub.
			// This might require additional rounds of hello.
			// See https://github.com/filecoin-project/go-filecoin/issues/1105
			node.ChainSynced.Done()
		}
		node.HelloSvc = hello.New(node.Host(), node.ChainReader.GenesisCid(), helloCallback, node.PorcelainAPI.ChainHead, node.NetworkName)

		// register the update function on the peer tracker now that we have a hello service
		node.PeerTracker.SetUpdateFn(func(ctx context.Context, p peer.ID) (*types.ChainInfo, error) {
			hmsg, err := node.HelloSvc.ReceiveHello(ctx, p)
			if err != nil {
				return nil, err
			}
			return types.NewChainInfo(p, hmsg.HeaviestTipSetCids, hmsg.HeaviestTipSetHeight), nil
		})

		// Subscribe to block pubsub after the initial sync completes.
		go func() {
			node.ChainSynced.Wait()

			// Log some information about the synced chain
			if ts, err := node.ChainReader.GetTipSet(node.ChainReader.GetHead()); err == nil {
				if height, err := ts.Height(); err == nil {
					log.Infof("initial chain sync complete! chain head height %d, tipset key %s, blocks %s\n", height, ts.Key(), ts.String())
				}
			}

			if syncCtx.Err() == nil {
				// Subscribe to block pubsub topic to learn about new chain heads.
				node.BlockSub, err = node.pubsubscribe(syncCtx, net.BlockTopic(node.NetworkName), node.processBlock)
				if err != nil {
					log.Error(err)
				}
			}
		}()

		// Subscribe to the message pubsub topic to learn about messages to mine into blocks.
		// TODO: defer this subscription until after mining (block production) is started:
		// https://github.com/filecoin-project/go-filecoin/issues/2145.
		// This is blocked by https://github.com/filecoin-project/go-filecoin/issues/2959, which
		// is necessary for message_propagate_test to start mining before testing this behaviour.
		node.MessageSub, err = node.pubsubscribe(syncCtx, net.MessageTopic(node.NetworkName), node.processMessage)
		if err != nil {
			return err
		}

		// Start heartbeats.
		if err := node.setupHeartbeatServices(ctx); err != nil {
			return errors.Wrap(err, "failed to start heartbeat services")
		}
	}

	return nil
}

// Subscribes a handler function to a pubsub topic.
func (node *Node) pubsubscribe(ctx context.Context, topic string, handler pubSubHandler) (pubsub.Subscription, error) {
	sub, err := node.PorcelainAPI.PubSubSubscribe(topic)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to subscribe to %s", topic)
	}
	go node.handleSubscription(ctx, sub, handler)
	return sub, nil
}

func (node *Node) setupHeartbeatServices(ctx context.Context) error {
	mag := func() address.Address {
		addr, err := node.MiningAddress()
		// the only error MiningAddress() returns is ErrNoMinerAddress.
		// if there is no configured miner address, simply send a zero
		// address across the wire.
		if err != nil {
			return address.Undef
		}
		return addr
	}

	// start the primary heartbeat service
	if len(node.Repo.Config().Heartbeat.BeatTarget) > 0 {
		hbs := metrics.NewHeartbeatService(node.Host(), node.ChainReader.GenesisCid(), node.Repo.Config().Heartbeat, node.PorcelainAPI.ChainHead, metrics.WithMinerAddressGetter(mag))
		go hbs.Start(ctx)
	}

	// check if we want to connect to an alert service. An alerting service is a heartbeat
	// service that can trigger alerts based on the contents of heatbeats.
	if alertTarget := os.Getenv("FIL_HEARTBEAT_ALERTS"); len(alertTarget) > 0 {
		ahbs := metrics.NewHeartbeatService(node.Host(), node.ChainReader.GenesisCid(), &config.HeartbeatConfig{
			BeatTarget:      alertTarget,
			BeatPeriod:      "10s",
			ReconnectPeriod: "10s",
			Nickname:        node.Repo.Config().Heartbeat.Nickname,
		}, node.PorcelainAPI.ChainHead, metrics.WithMinerAddressGetter(mag))
		go ahbs.Start(ctx)
	}
	return nil
}

func (node *Node) setupMining(ctx context.Context) error {
	// initialize a sector builder
	sectorBuilder, err := initSectorBuilderForNode(ctx, node)
	if err != nil {
		return errors.Wrap(err, "failed to initialize sector builder")
	}
	node.sectorBuilder = sectorBuilder

	return nil
}

func (node *Node) setIsMining(isMining bool) {
	node.mining.Lock()
	defer node.mining.Unlock()
	node.mining.isMining = isMining
}

func (node *Node) handleNewMiningOutput(ctx context.Context, miningOutCh <-chan mining.Output) {
	defer func() {
		node.miningDoneWg.Done()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case output, ok := <-miningOutCh:
			if !ok {
				return
			}
			if output.Err != nil {
				log.Errorf("stopping mining. error: %s", output.Err.Error())
				node.StopMining(context.Background())
			} else {
				node.miningDoneWg.Add(1)
				go func() {
					if node.IsMining() {
						node.AddNewlyMinedBlock(ctx, output.NewBlock)
					}
					node.miningDoneWg.Done()
				}()
			}
		}
	}

}

func (node *Node) handleNewChainHeads(ctx context.Context, prevHead types.TipSet) {
	node.HeaviestTipSetCh = node.ChainReader.HeadEvents().Sub(chain.NewHeadTopic)
	handler := core.NewHandler(node.Inbox, node.Outbox, node.ChainReader, prevHead)

	for {
		select {
		case ts, ok := <-node.HeaviestTipSetCh:
			if !ok {
				return
			}
			newHead, ok := ts.(types.TipSet)
			if !ok {
				log.Warning("non-tipset published on heaviest tipset channel")
				continue
			}

			if err := handler.HandleNewHead(ctx, newHead); err != nil {
				log.Error(err)
			}

			if node.StorageMiner != nil {
				if err := node.StorageMiner.OnNewHeaviestTipSet(newHead); err != nil {
					log.Error(err)
				}
			}
			if node.StorageFaultSlasher != nil {
				if err := node.StorageFaultSlasher.OnNewHeaviestTipSet(ctx, newHead); err != nil {
					log.Error(err)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (node *Node) cancelSubscriptions() {
	if node.cancelChainSync != nil {
		node.cancelChainSync()
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
	node.ChainReader.HeadEvents().Unsub(node.HeaviestTipSetCh)
	node.StopMining(ctx)

	node.cancelSubscriptions()
	node.ChainReader.Stop()

	if node.SectorBuilder() != nil {
		if err := node.SectorBuilder().Close(); err != nil {
			fmt.Printf("error closing sector builder: %s\n", err)
		}
		node.sectorBuilder = nil
	}

	if err := node.Host().Close(); err != nil {
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
	log.Debugf("Got a newly mined block from the mining worker: %s", b)
	if err := node.AddNewBlock(ctx, b); err != nil {
		log.Warningf("error adding new mined block: %s. err: %s", b.Cid().String(), err.Error())
	}
}

// MiningAddress returns the address of the mining actor mining on behalf of
// the node.
func (node *Node) MiningAddress() (address.Address, error) {
	addr := node.Repo.Config().Mining.MinerAddress
	if addr.Empty() {
		return address.Undef, ErrNoMinerAddress
	}

	return addr, nil
}

// MiningTimes returns the configured time it takes to mine a block, and also
// the mining delay duration, which is currently a fixed fraction of block time.
// Note this is mocked behavior, in production this time is determined by how
// long it takes to generate PoSTs.
func (node *Node) MiningTimes() (time.Duration, time.Duration) {
	blockTime := node.PorcelainAPI.BlockTime()
	mineDelay := blockTime / mining.MineDelayConversionFactor
	return blockTime, mineDelay
}

// SetupMining initializes all the functionality the node needs to start mining.
// This method is idempotent.
func (node *Node) SetupMining(ctx context.Context) error {
	// ensure we have a miner actor before we even consider mining
	minerAddr, err := node.MiningAddress()
	if err != nil {
		return errors.Wrap(err, "failed to get mining address")
	}
	_, err = node.PorcelainAPI.ActorGet(ctx, minerAddr)
	if err != nil {
		return errors.Wrap(err, "failed to get miner actor")
	}

	// ensure we have a sector builder
	if node.SectorBuilder() == nil {
		if err := node.setupMining(ctx); err != nil {
			return err
		}
	}

	// ensure we have a mining worker
	if node.MiningWorker == nil {
		if node.MiningWorker, err = node.CreateMiningWorker(ctx); err != nil {
			return err
		}
	}

	// ensure we have a storage miner
	if node.StorageMiner == nil {
		storageMiner, _, err := initStorageMinerForNode(ctx, node)
		if err != nil {
			return errors.Wrap(err, "failed to initialize storage miner")
		}
		node.StorageMiner = storageMiner
	}

	return nil
}

// StartMining causes the node to start feeding blocks to the mining worker and initializes
// the SectorBuilder for the mining address.
func (node *Node) StartMining(ctx context.Context) error {
	if node.IsMining() {
		return errors.New("Node is already mining")
	}

	err := node.SetupMining(ctx)
	if err != nil {
		return err
	}

	minerAddr, err := node.MiningAddress()
	if err != nil {
		return errors.Wrap(err, "failed to get mining address")
	}

	minerOwnerAddr, err := node.PorcelainAPI.MinerGetOwnerAddress(ctx, minerAddr)
	if err != nil {
		return errors.Wrapf(err, "failed to get mining owner address for miner %s", minerAddr)
	}

	_, mineDelay := node.MiningTimes()

	if node.MiningScheduler == nil {
		node.MiningScheduler = mining.NewScheduler(node.MiningWorker, mineDelay, node.PorcelainAPI.ChainHead)
	} else if node.MiningScheduler.IsStarted() {
		return fmt.Errorf("miner scheduler already started")
	}

	var miningCtx context.Context
	miningCtx, node.cancelMining = context.WithCancel(context.Background())

	outCh, doneWg := node.MiningScheduler.Start(miningCtx)

	node.miningDoneWg = doneWg
	node.AddNewlyMinedBlock = node.addNewlyMinedBlock
	node.miningDoneWg.Add(1)
	go node.handleNewMiningOutput(miningCtx, outCh)

	// initialize the storage fault slasher
	node.StorageFaultSlasher = storage.NewFaultSlasher(
		node.PorcelainAPI,
		node.Outbox,
		storage.DefaultFaultSlasherGasPrice,
		storage.DefaultFaultSlasherGasLimit)

	// loop, turning sealing-results into commitSector messages to be included
	// in the chain
	go func() {
		for {
			select {
			case result := <-node.SectorBuilder().SectorSealResults():
				if result.SealingErr != nil {
					log.Errorf("failed to seal sector with id %d: %s", result.SectorID, result.SealingErr.Error())
				} else if result.SealingResult != nil {

					// TODO: determine these algorithmically by simulating call and querying historical prices
					gasPrice := types.NewGasPrice(1)
					gasUnits := types.NewGasUnits(300)

					val := result.SealingResult

					// look up miner worker address. If this fails, something is really wrong
					// so we bail and don't commit sectors.
					workerAddr, err := node.PorcelainAPI.MinerGetWorkerAddress(miningCtx, minerAddr, node.ChainReader.GetHead())
					if err != nil {
						log.Errorf("failed to get worker address %s", err)
						continue
					}

					// This call can fail due to, e.g. nonce collisions. Our miners existence depends on this.
					// We should deal with this, but MessageSendWithRetry is problematic.
					msgCid, err := node.PorcelainAPI.MessageSend(
						miningCtx,
						workerAddr,
						minerAddr,
						types.ZeroAttoFIL,
						gasPrice,
						gasUnits,
						"commitSector",
						val.SectorID,
						val.CommD[:],
						val.CommR[:],
						val.CommRStar[:],
						val.Proof[:],
					)

					if err != nil {
						log.Errorf("failed to send commitSector message from %s to %s for sector with id %d: %s", minerOwnerAddr, minerAddr, val.SectorID, err)
						continue
					}

					node.StorageMiner.OnCommitmentSent(val, msgCid, nil)
				}
			case <-miningCtx.Done():
				return
			}
		}
	}()

	// schedules sealing of staged piece-data
	if node.Repo.Config().Mining.AutoSealIntervalSeconds > 0 {
		go func() {
			for {
				select {
				case <-miningCtx.Done():
					return
				case <-time.After(time.Duration(node.Repo.Config().Mining.AutoSealIntervalSeconds) * time.Second):
					log.Info("auto-seal has been triggered")
					if err := node.SectorBuilder().SealAllStagedSectors(miningCtx); err != nil {
						log.Errorf("scheduler received error from node.SectorBuilder.SealAllStagedSectors (%s) - exiting", err.Error())
						return
					}
				}
			}
		}()
	} else {
		log.Debug("auto-seal is disabled")
	}
	node.setIsMining(true)

	return nil
}

// NetworkNameFromGenesis retrieves the name of the current network from the genesis block.
// The network name can not change while this node is running. Since the network name determines
// the protocol version, we must retrieve it at genesis where the protocol is known.
func networkNameFromGenesis(ctx context.Context, chainStore *chain.Store, bs bstore.Blockstore) (string, error) {
	st, err := chainStore.GetGenesisState(ctx)
	if err != nil {
		return "", errors.Wrap(err, "could not get genesis state")
	}

	vms := vm.NewStorageMap(bs)
	res, _, err := consensus.CallQueryMethod(ctx, st, vms, address.InitAddress, "getNetwork", nil, address.Undef, types.NewBlockHeight(0))
	if err != nil {
		return "", errors.Wrap(err, "error querying for network name")
	}

	return string(res[0]), nil
}

func initSectorBuilderForNode(ctx context.Context, node *Node) (sectorbuilder.SectorBuilder, error) {
	minerAddr, err := node.MiningAddress()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node's mining address")
	}

	sectorSize, err := node.PorcelainAPI.MinerGetSectorSize(ctx, minerAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get sector size for miner w/address %s", minerAddr.String())
	}

	lastUsedSectorID, err := node.PorcelainAPI.MinerGetLastCommittedSectorID(ctx, minerAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get last used sector id for miner w/address %s", minerAddr.String())
	}

	// TODO: Currently, weconfigure the RustSectorBuilder to store its
	// metadata in the staging directory, it should be in its own directory.
	//
	// Tracked here: https://github.com/filecoin-project/rust-fil-proofs/issues/402
	repoPath, err := node.Repo.Path()
	if err != nil {
		return nil, err
	}
	sectorDir, err := paths.GetSectorPath(node.Repo.Config().SectorBase.RootDir, repoPath)
	if err != nil {
		return nil, err
	}

	stagingDir, err := paths.StagingDir(sectorDir)
	if err != nil {
		return nil, err
	}

	sealedDir, err := paths.SealedDir(sectorDir)
	if err != nil {
		return nil, err
	}
	cfg := sectorbuilder.RustSectorBuilderConfig{
		BlockService:     node.blockservice,
		LastUsedSectorID: lastUsedSectorID,
		MetadataDir:      stagingDir,
		MinerAddr:        minerAddr,
		SealedSectorDir:  sealedDir,
		StagedSectorDir:  stagingDir,
		SectorClass:      types.NewSectorClass(sectorSize),
	}

	sb, err := sectorbuilder.NewRustSectorBuilder(cfg)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to initialize sector builder for miner %s", minerAddr.String()))
	}

	return sb, nil
}

// initStorageMinerForNode initializes the storage miner, returning the miner, the miner owner address (to be
// passed to storage fault slasher) and any error
func initStorageMinerForNode(ctx context.Context, node *Node) (*storage.Miner, address.Address, error) {
	minerAddr, err := node.MiningAddress()
	if err != nil {
		return nil, address.Undef, errors.Wrap(err, "failed to get node's mining address")
	}

	ownerAddress, err := node.PorcelainAPI.MinerGetOwnerAddress(ctx, minerAddr)
	if err != nil {
		return nil, address.Undef, errors.Wrap(err, "no mining owner available, skipping storage miner setup")
	}

	workerAddress, err := node.PorcelainAPI.MinerGetWorkerAddress(ctx, minerAddr, node.ChainReader.GetHead())
	if err != nil {
		return nil, address.Undef, errors.Wrap(err, "failed to fetch miner's worker address")
	}

	sectorSize, err := node.PorcelainAPI.MinerGetSectorSize(ctx, minerAddr)
	if err != nil {
		return nil, address.Undef, errors.Wrap(err, "failed to fetch miner's sector size")
	}

	prover := storage.NewProver(minerAddr, sectorSize, node.PorcelainAPI, node.PorcelainAPI)

	miner, err := storage.NewMiner(
		minerAddr,
		ownerAddress,
		prover,
		sectorSize,
		node,
		node.Repo.DealsDatastore(),
		node.PorcelainAPI)
	if err != nil {
		return nil, address.Undef, errors.Wrap(err, "failed to instantiate storage miner")
	}

	return miner, workerAddress, nil
}

// StopMining stops mining on new blocks.
func (node *Node) StopMining(ctx context.Context) {
	node.setIsMining(false)

	if node.cancelMining != nil {
		node.cancelMining()
	}

	if node.miningDoneWg != nil {
		node.miningDoneWg.Wait()
	}

	// TODO: stop node.StorageMiner
}

func (node *Node) handleSubscription(ctx context.Context, sub pubsub.Subscription, handler pubSubHandler) {
	for {
		received, err := sub.Next(ctx)
		if err != nil {
			if ctx.Err() != context.Canceled {
				log.Errorf("error reading message from topic %s: %s", sub.Topic(), err)
			}
			return
		}

		if err := handler(ctx, received); err != nil {
			handlerName := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
			if vmerr.ShouldRevert(err) {
				log.Infof("error in handler %s for topic %s: %s", handlerName, sub.Topic(), err)
			} else if err != context.Canceled {
				log.Errorf("error in handler %s for topic %s: %s", handlerName, sub.Topic(), err)
			}
		}
	}
}

// setupProtocols creates protocol clients and miners, then sets the node's APIs
// for each
func (node *Node) setupProtocols() error {
	_, mineDelay := node.MiningTimes()
	blockMiningAPI := block.New(
		node.MiningAddress,
		node.AddNewBlock,
		node.ChainReader,
		node.IsMining,
		mineDelay,
		node.SetupMining,
		node.StartMining,
		node.StopMining,
		node.GetMiningWorker)

	node.BlockMiningAPI = &blockMiningAPI

	// set up retrieval client and api
	retapi := retrieval.NewAPI(retrieval.NewClient(node.host, node.PorcelainAPI))
	node.RetrievalAPI = &retapi

	// set up storage client and api
	smc := storage.NewClient(node.host, node.PorcelainAPI)
	smcAPI := storage.NewAPI(smc)
	node.StorageAPI = &smcAPI
	return nil
}

// GetMiningWorker ensures mining is setup and then returns the worker
func (node *Node) GetMiningWorker(ctx context.Context) (mining.Worker, error) {
	if err := node.SetupMining(ctx); err != nil {
		return nil, err
	}
	return node.MiningWorker, nil
}

// CreateMiningWorker creates a mining.Worker for the node using the configured
// getStateTree, getWeight, and getAncestors functions for the node
func (node *Node) CreateMiningWorker(ctx context.Context) (mining.Worker, error) {
	processor := consensus.NewDefaultProcessor()

	minerAddr, err := node.MiningAddress()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get mining address")
	}

	minerOwnerAddr, err := node.PorcelainAPI.MinerGetOwnerAddress(ctx, minerAddr)
	if err != nil {
		log.Errorf("could not get owner address of miner actor")
		return nil, err
	}
	return mining.NewDefaultWorker(mining.WorkerParameters{
		API: node.PorcelainAPI,

		MinerAddr:      minerAddr,
		MinerOwnerAddr: minerOwnerAddr,
		WorkerSigner:   node.Wallet,

		GetStateTree: node.getStateTree,
		GetWeight:    node.getWeight,
		GetAncestors: node.getAncestors,
		Election:     consensus.ElectionMachine{},
		TicketGen:    consensus.TicketMachine{},

		MessageSource: node.Inbox.Pool(),
		MessageStore:  node.MessageStore,
		Processor:     processor,
		PowerTable:    node.PowerTable,
		Blockstore:    node.Blockstore,
		Clock:         node.Clock,
	}), nil
}

// getStateTree is the default GetStateTree function for the mining worker.
func (node *Node) getStateTree(ctx context.Context, ts types.TipSet) (state.Tree, error) {
	return node.ChainReader.GetTipSetState(ctx, ts.Key())
}

// getWeight is the default GetWeight function for the mining worker.
func (node *Node) getWeight(ctx context.Context, ts types.TipSet) (uint64, error) {
	parent, err := ts.Parents()
	if err != nil {
		return uint64(0), err
	}
	// TODO handle genesis cid more gracefully
	if parent.Len() == 0 {
		return node.Consensus.Weight(ctx, ts, nil)
	}
	pSt, err := node.ChainReader.GetTipSetState(ctx, parent)
	if err != nil {
		return uint64(0), err
	}
	return node.Consensus.Weight(ctx, ts, pSt)
}

// getAncestors is the default GetAncestors function for the mining worker.
func (node *Node) getAncestors(ctx context.Context, ts types.TipSet, newBlockHeight *types.BlockHeight) ([]types.TipSet, error) {
	ancestorHeight := types.NewBlockHeight(consensus.AncestorRoundsNeeded)
	return chain.GetRecentAncestors(ctx, ts, node.ChainReader, newBlockHeight, ancestorHeight, sampling.LookbackParameter)
}

// -- Accessors

// Host returns the nodes host.
func (node *Node) Host() host.Host {
	return node.host
}

// SectorBuilder returns the nodes sectorBuilder.
func (node *Node) SectorBuilder() sectorbuilder.SectorBuilder {
	return node.sectorBuilder
}

// BlockService returns the nodes blockservice.
func (node *Node) BlockService() bserv.BlockService {
	return node.blockservice
}

// CborStore returns the nodes cborStore.
func (node *Node) CborStore() *hamt.CborIpldStore {
	return node.cborStore
}

// IsMining returns a boolean indicating whether the node is mining blocks.
func (node *Node) IsMining() bool {
	node.mining.Lock()
	defer node.mining.Unlock()
	return node.mining.isMining
}
