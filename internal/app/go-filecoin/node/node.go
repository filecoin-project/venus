package node

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"time"

	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paths"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics"
	"github.com/filecoin-project/go-filecoin/internal/pkg/mining"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net/pubsub"
	mining_protocol "github.com/filecoin-project/go-filecoin/internal/pkg/protocol/mining"
	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/retrieval"
	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/version"
	vmerr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
)

var log = logging.Logger("node") // nolint: deadcode

var (
	// ErrNoMinerAddress is returned when the node is not configured to have any miner addresses.
	ErrNoMinerAddress = errors.New("no miner addresses configured")
)

// Node represents a full Filecoin node.
type Node struct {
	// OfflineMode, when true, disables libp2p.
	OfflineMode bool

	// Clock is a clock used by the node for time.
	Clock clock.Clock

	// Repo is the repo this node was created with.
	//
	// It contains all persistent artifacts of the filecoin node.
	Repo repo.Repo

	PorcelainAPI *porcelain.API

	//
	// Core services
	//

	Blockstore   BlockstoreSubmodule
	Network      NetworkSubmodule
	Blockservice BlockserviceSubmodule
	Discovery    DiscoverySubmodule

	//
	// Subsystems
	//

	Chain         ChainSubmodule
	BlockMining   BlockMiningSubmodule
	SectorStorage SectorBuilderSubmodule

	//
	// Supporting services
	//

	Wallet            WalletSubmodule
	Messaging         MessagingSubmodule
	StorageNetworking StorageNetworkingSubmodule

	//
	// Protocols
	//

	VersionTable      *version.ProtocolVersionTable
	StorageProtocol   StorageProtocolSubmodule
	RetrievalProtocol RetrievalProtocolSubmodule

	//
	// Additional services
	//

	FaultSlasher FaultSlasherSubmodule
}

// Start boots up the node.
func (node *Node) Start(ctx context.Context) error {
	if err := metrics.RegisterPrometheusEndpoint(node.Repo.Config().Observability.Metrics); err != nil {
		return errors.Wrap(err, "failed to setup metrics")
	}

	if err := metrics.RegisterJaeger(node.Network.host.ID().Pretty(), node.Repo.Config().Observability.Tracing); err != nil {
		return errors.Wrap(err, "failed to setup tracing")
	}

	var err error
	if err = node.Chain.ChainReader.Load(ctx); err != nil {
		return err
	}

	// Only set these up if there is a miner configured.
	if _, err := node.MiningAddress(); err == nil {
		if err := node.setupSectorBuilder(ctx); err != nil {
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
	node.RetrievalProtocol.RetrievalMiner = retrieval.NewMiner(node)

	var syncCtx context.Context
	syncCtx, node.Chain.cancelChainSync = context.WithCancel(context.Background())

	// Wire up propagation of new chain heads from the chain store to other components.
	head, err := node.PorcelainAPI.ChainHead()
	if err != nil {
		return errors.Wrap(err, "failed to get chain head")
	}
	go node.handleNewChainHeads(syncCtx, head)

	if !node.OfflineMode {

		// Start syncing dispatch
		node.Chain.SyncDispatch.Start(context.Background())

		// Start node discovery
		if err := node.Discovery.Start(node); err != nil {
			return err
		}

		// Subscribe to block pubsub after the initial sync completes.
		go func() {
			node.Chain.ChainSynced.Wait()

			// Log some information about the synced chain
			if ts, err := node.Chain.ChainReader.GetTipSet(node.Chain.ChainReader.GetHead()); err == nil {
				if height, err := ts.Height(); err == nil {
					log.Infof("initial chain sync complete! chain head height %d, tipset key %s, blocks %s\n", height, ts.Key(), ts.String())
				}
			}

			if syncCtx.Err() == nil {
				// Subscribe to block pubsub topic to learn about new chain heads.
				node.Chain.BlockSub, err = node.pubsubscribe(syncCtx, net.BlockTopic(node.Network.NetworkName), node.processBlock)
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
		node.Messaging.MessageSub, err = node.pubsubscribe(syncCtx, net.MessageTopic(node.Network.NetworkName), node.processMessage)
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
		hbs := metrics.NewHeartbeatService(node.Host(), node.Chain.ChainReader.GenesisCid(), node.Repo.Config().Heartbeat, node.PorcelainAPI.ChainHead, metrics.WithMinerAddressGetter(mag))
		go hbs.Start(ctx)
	}

	// check if we want to connect to an alert service. An alerting service is a heartbeat
	// service that can trigger alerts based on the contents of heatbeats.
	if alertTarget := os.Getenv("FIL_HEARTBEAT_ALERTS"); len(alertTarget) > 0 {
		ahbs := metrics.NewHeartbeatService(node.Host(), node.Chain.ChainReader.GenesisCid(), &config.HeartbeatConfig{
			BeatTarget:      alertTarget,
			BeatPeriod:      "10s",
			ReconnectPeriod: "10s",
			Nickname:        node.Repo.Config().Heartbeat.Nickname,
		}, node.PorcelainAPI.ChainHead, metrics.WithMinerAddressGetter(mag))
		go ahbs.Start(ctx)
	}
	return nil
}

func (node *Node) setupSectorBuilder(ctx context.Context) error {
	// initialize a sector builder
	sectorBuilder, err := initSectorBuilderForNode(ctx, node)
	if err != nil {
		return errors.Wrap(err, "failed to initialize sector builder")
	}
	node.SectorStorage.sectorBuilder = sectorBuilder

	return nil
}

func (node *Node) setIsMining(isMining bool) {
	node.BlockMining.mining.Lock()
	defer node.BlockMining.mining.Unlock()
	node.BlockMining.mining.isMining = isMining
}

func (node *Node) handleNewMiningOutput(ctx context.Context, miningOutCh <-chan mining.Output) {
	defer func() {
		node.BlockMining.miningDoneWg.Done()
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
				node.BlockMining.miningDoneWg.Add(1)
				go func() {
					if node.IsMining() {
						node.BlockMining.AddNewlyMinedBlock(ctx, output.NewBlock)
					}
					node.BlockMining.miningDoneWg.Done()
				}()
			}
		}
	}

}

func (node *Node) handleNewChainHeads(ctx context.Context, prevHead block.TipSet) {
	node.Chain.HeaviestTipSetCh = node.Chain.ChainReader.HeadEvents().Sub(chain.NewHeadTopic)
	handler := message.NewHeadHandler(node.Messaging.Inbox, node.Messaging.Outbox, node.Chain.ChainReader, prevHead)

	for {
		select {
		case ts, ok := <-node.Chain.HeaviestTipSetCh:
			if !ok {
				return
			}
			newHead, ok := ts.(block.TipSet)
			if !ok {
				log.Warn("non-tipset published on heaviest tipset channel")
				continue
			}

			if err := handler.HandleNewHead(ctx, newHead); err != nil {
				log.Error(err)
			}

			if node.StorageProtocol.StorageMiner != nil {
				if _, err := node.StorageProtocol.StorageMiner.OnNewHeaviestTipSet(newHead); err != nil {
					log.Error(err)
				}
			}
			if node.FaultSlasher.StorageFaultSlasher != nil {
				if err := node.FaultSlasher.StorageFaultSlasher.OnNewHeaviestTipSet(ctx, newHead); err != nil {
					log.Error(err)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (node *Node) cancelSubscriptions() {
	if node.Chain.cancelChainSync != nil {
		node.Chain.cancelChainSync()
	}

	if node.Chain.BlockSub != nil {
		node.Chain.BlockSub.Cancel()
		node.Chain.BlockSub = nil
	}

	if node.Messaging.MessageSub != nil {
		node.Messaging.MessageSub.Cancel()
		node.Messaging.MessageSub = nil
	}
}

// Stop initiates the shutdown of the node.
func (node *Node) Stop(ctx context.Context) {
	node.Chain.ChainReader.HeadEvents().Unsub(node.Chain.HeaviestTipSetCh)
	node.StopMining(ctx)

	node.cancelSubscriptions()
	node.Chain.ChainReader.Stop()

	if node.SectorBuilder() != nil {
		if err := node.SectorBuilder().Close(); err != nil {
			fmt.Printf("error closing sector builder: %s\n", err)
		}
		node.SectorStorage.sectorBuilder = nil
	}

	if err := node.Host().Close(); err != nil {
		fmt.Printf("error closing host: %s\n", err)
	}

	if err := node.Repo.Close(); err != nil {
		fmt.Printf("error closing repo: %s\n", err)
	}

	node.Discovery.Bootstrapper.Stop()

	fmt.Println("stopping filecoin :(")
}

type newBlockFunc func(context.Context, *block.Block)

func (node *Node) addNewlyMinedBlock(ctx context.Context, b *block.Block) {
	log.Debugf("Got a newly mined block from the mining worker: %s", b)
	if err := node.AddNewBlock(ctx, b); err != nil {
		log.Warnf("error adding new mined block: %s. err: %s", b.Cid().String(), err.Error())
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
		if err := node.setupSectorBuilder(ctx); err != nil {
			return err
		}
	}

	// ensure we have a mining worker
	if node.BlockMining.MiningWorker == nil {
		if node.BlockMining.MiningWorker, err = node.CreateMiningWorker(ctx); err != nil {
			return err
		}
	}

	// ensure we have a storage miner
	if node.StorageProtocol.StorageMiner == nil {
		storageMiner, _, err := initStorageMinerForNode(ctx, node)
		if err != nil {
			return errors.Wrap(err, "failed to initialize storage miner")
		}
		node.StorageProtocol.StorageMiner = storageMiner
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

	if node.BlockMining.MiningScheduler == nil {
		node.BlockMining.MiningScheduler = mining.NewScheduler(node.BlockMining.MiningWorker, mineDelay, node.PorcelainAPI.ChainHead)
	} else if node.BlockMining.MiningScheduler.IsStarted() {
		return fmt.Errorf("miner scheduler already started")
	}

	var miningCtx context.Context
	miningCtx, node.BlockMining.cancelMining = context.WithCancel(context.Background())

	outCh, doneWg := node.BlockMining.MiningScheduler.Start(miningCtx)

	node.BlockMining.miningDoneWg = doneWg
	node.BlockMining.AddNewlyMinedBlock = node.addNewlyMinedBlock
	node.BlockMining.miningDoneWg.Add(1)
	go node.handleNewMiningOutput(miningCtx, outCh)

	// initialize the storage fault slasher
	node.FaultSlasher.StorageFaultSlasher = storage.NewFaultSlasher(
		node.PorcelainAPI,
		node.Messaging.Outbox,
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
					workerAddr, err := node.PorcelainAPI.MinerGetWorkerAddress(miningCtx, minerAddr, node.Chain.ChainReader.GetHead())
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

					node.StorageProtocol.StorageMiner.OnCommitmentSent(val, msgCid, nil)
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
						log.Errorf("scheduler received error from node.SectorStorage.sectorBuilder.SealAllStagedSectors (%s) - exiting", err.Error())
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
		BlockService:     node.Blockservice.blockservice,
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

	workerAddress, err := node.PorcelainAPI.MinerGetWorkerAddress(ctx, minerAddr, node.Chain.ChainReader.GetHead())
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

	if node.BlockMining.cancelMining != nil {
		node.BlockMining.cancelMining()
	}

	if node.BlockMining.miningDoneWg != nil {
		node.BlockMining.miningDoneWg.Wait()
	}

	// TODO: stop node.StorageProtocol.StorageMiner
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
	blockMiningAPI := mining_protocol.New(
		node.MiningAddress,
		node.AddNewBlock,
		node.Chain.ChainReader,
		node.IsMining,
		mineDelay,
		node.SetupMining,
		node.StartMining,
		node.StopMining,
		node.GetMiningWorker)

	node.BlockMining.BlockMiningAPI = &blockMiningAPI

	// set up retrieval client and api
	retapi := retrieval.NewAPI(retrieval.NewClient(node.Network.host, node.PorcelainAPI))
	node.RetrievalProtocol.RetrievalAPI = &retapi

	// set up storage client and api
	smc := storage.NewClient(node.Network.host, node.PorcelainAPI)
	smcAPI := storage.NewAPI(smc)
	node.StorageProtocol.StorageAPI = &smcAPI
	return nil
}

// GetMiningWorker ensures mining is setup and then returns the worker
func (node *Node) GetMiningWorker(ctx context.Context) (mining.Worker, error) {
	if err := node.SetupMining(ctx); err != nil {
		return nil, err
	}
	return node.BlockMining.MiningWorker, nil
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
		WorkerSigner:   node.Wallet.Wallet,

		GetStateTree: node.getStateTree,
		GetWeight:    node.getWeight,
		GetAncestors: node.getAncestors,
		Election:     consensus.ElectionMachine{},
		TicketGen:    consensus.TicketMachine{},

		MessageSource: node.Messaging.Inbox.Pool(),
		MessageStore:  node.Chain.MessageStore,
		Processor:     processor,
		Blockstore:    node.Blockstore.Blockstore,
		Clock:         node.Clock,
	}), nil
}

// getStateTree is the default GetStateTree function for the mining worker.
func (node *Node) getStateTree(ctx context.Context, ts block.TipSet) (state.Tree, error) {
	return node.Chain.ChainReader.GetTipSetState(ctx, ts.Key())
}

// getWeight is the default GetWeight function for the mining worker.
func (node *Node) getWeight(ctx context.Context, ts block.TipSet) (uint64, error) {
	h, err := ts.Height()
	if err != nil {
		return 0, err
	}
	var wFun func(context.Context, block.TipSet, cid.Cid) (uint64, error)
	v, err := node.VersionTable.VersionAt(types.NewBlockHeight(h))
	if err != nil {
		return 0, err
	}
	if v >= version.Protocol1 {
		wFun = node.Chain.ChainSelector.NewWeight
	} else {
		wFun = node.Chain.ChainSelector.Weight
	}

	parent, err := ts.Parents()
	if err != nil {
		return uint64(0), err
	}
	// TODO handle genesis cid more gracefully
	if parent.Len() == 0 {
		return wFun(ctx, ts, cid.Undef)
	}
	root, err := node.Chain.ChainReader.GetTipSetStateRoot(parent)
	if err != nil {
		return uint64(0), err
	}
	return wFun(ctx, ts, root)
}

// getAncestors is the default GetAncestors function for the mining worker.
func (node *Node) getAncestors(ctx context.Context, ts block.TipSet, newBlockHeight *types.BlockHeight) ([]block.TipSet, error) {
	ancestorHeight := newBlockHeight.Sub(types.NewBlockHeight(consensus.AncestorRoundsNeeded))
	return chain.GetRecentAncestors(ctx, ts, node.Chain.ChainReader, ancestorHeight)
}

// -- Accessors

// Host returns the nodes host.
func (node *Node) Host() host.Host {
	return node.Network.host
}

// SectorBuilder returns the nodes sectorBuilder.
func (node *Node) SectorBuilder() sectorbuilder.SectorBuilder {
	return node.SectorStorage.sectorBuilder
}

// BlockService returns the nodes blockservice.
func (node *Node) BlockService() bserv.BlockService {
	return node.Blockservice.blockservice
}

// CborStore returns the nodes cborStore.
func (node *Node) CborStore() *hamt.CborIpldStore {
	return node.Blockstore.cborStore
}

// IsMining returns a boolean indicating whether the node is mining blocks.
func (node *Node) IsMining() bool {
	node.BlockMining.mining.Lock()
	defer node.BlockMining.mining.Unlock()
	return node.BlockMining.mining.isMining
}
