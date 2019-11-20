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

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/internal/submodule"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paths"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics"
	"github.com/filecoin-project/go-filecoin/internal/pkg/mining"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net/pubsub"
	mining_protocol "github.com/filecoin-project/go-filecoin/internal/pkg/protocol/mining"
	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/retrieval"
	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/version"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	vmerr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
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

	Blockstore   submodule.BlockstoreSubmodule
	network      submodule.NetworkSubmodule
	Blockservice submodule.BlockServiceSubmodule
	Discovery    submodule.DiscoverySubmodule

	//
	// Subsystems
	//

	chain         submodule.ChainSubmodule
	syncer        submodule.SyncerSubmodule
	BlockMining   submodule.BlockMiningSubmodule
	SectorStorage submodule.SectorBuilderSubmodule

	//
	// Supporting services
	//

	Wallet            submodule.WalletSubmodule
	Messaging         submodule.MessagingSubmodule
	StorageNetworking submodule.StorageNetworkingSubmodule

	//
	// Protocols
	//

	VersionTable      *version.ProtocolVersionTable
	StorageProtocol   submodule.StorageProtocolSubmodule
	RetrievalProtocol submodule.RetrievalProtocolSubmodule
}

// Start boots up the node.
func (node *Node) Start(ctx context.Context) error {
	if err := metrics.RegisterPrometheusEndpoint(node.Repo.Config().Observability.Metrics); err != nil {
		return errors.Wrap(err, "failed to setup metrics")
	}

	if err := metrics.RegisterJaeger(node.network.Host.ID().Pretty(), node.Repo.Config().Observability.Tracing); err != nil {
		return errors.Wrap(err, "failed to setup tracing")
	}

	err := node.chain.Start(ctx, node)
	if err != nil {
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
	syncCtx, node.syncer.CancelChainSync = context.WithCancel(context.Background())

	// Wire up propagation of new chain heads from the chain store to other components.
	head, err := node.PorcelainAPI.ChainHead()
	if err != nil {
		return errors.Wrap(err, "failed to get chain head")
	}
	go node.handleNewChainHeads(syncCtx, head)

	if !node.OfflineMode {

		// Subscribe to block pubsub topic to learn about new chain heads.
		node.syncer.BlockSub, err = node.pubsubscribe(syncCtx, node.syncer.BlockTopic, node.processBlock)
		if err != nil {
			log.Error(err)
		}

		// Subscribe to the message pubsub topic to learn about messages to mine into blocks.
		// TODO: defer this subscription until after mining (block production) is started:
		// https://github.com/filecoin-project/go-filecoin/issues/2145.
		// This is blocked by https://github.com/filecoin-project/go-filecoin/issues/2959, which
		// is necessary for message_propagate_test to start mining before testing this behaviour.
		node.Messaging.MessageSub, err = node.pubsubscribe(syncCtx, node.Messaging.MessageTopic, node.processMessage)
		if err != nil {
			return err
		}

		if err := node.setupHeartbeatServices(ctx); err != nil {
			return errors.Wrap(err, "failed to start heartbeat services")
		}

		// Start node discovery
		if err := node.Discovery.Start(node); err != nil {
			return err
		}

		if err := node.syncer.Start(syncCtx, node); err != nil {
			return err
		}
	}

	return nil
}

// Subscribes a handler function to a pubsub topic.
func (node *Node) pubsubscribe(ctx context.Context, topic *pubsub.Topic, handler pubSubHandler) (pubsub.Subscription, error) {
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to subscribe")
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
		hbs := metrics.NewHeartbeatService(node.Host(), node.chain.ChainReader.GenesisCid(), node.Repo.Config().Heartbeat, node.PorcelainAPI.ChainHead, metrics.WithMinerAddressGetter(mag))
		go hbs.Start(ctx)
	}

	// check if we want to connect to an alert service. An alerting service is a heartbeat
	// service that can trigger alerts based on the contents of heatbeats.
	if alertTarget := os.Getenv("FIL_HEARTBEAT_ALERTS"); len(alertTarget) > 0 {
		ahbs := metrics.NewHeartbeatService(node.Host(), node.chain.ChainReader.GenesisCid(), &config.HeartbeatConfig{
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
	node.SectorStorage.SectorBuilder = sectorBuilder

	return nil
}

func (node *Node) setIsMining(isMining bool) {
	node.BlockMining.Mining.Lock()
	defer node.BlockMining.Mining.Unlock()
	node.BlockMining.Mining.IsMining = isMining
}

func (node *Node) handleNewMiningOutput(ctx context.Context, miningOutCh <-chan mining.Output) {
	defer func() {
		node.BlockMining.MiningDoneWg.Done()
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
				node.BlockMining.MiningDoneWg.Add(1)
				go func() {
					if node.IsMining() {
						node.BlockMining.AddNewlyMinedBlock(ctx, output.NewBlock)
					}
					node.BlockMining.MiningDoneWg.Done()
				}()
			}
		}
	}

}

func (node *Node) handleNewChainHeads(ctx context.Context, prevHead block.TipSet) {
	node.chain.HeaviestTipSetCh = node.chain.ChainReader.HeadEvents().Sub(chain.NewHeadTopic)
	handler := message.NewHeadHandler(node.Messaging.Inbox, node.Messaging.Outbox, node.chain.ChainReader, prevHead)

	for {
		select {
		case ts, ok := <-node.chain.HeaviestTipSetCh:
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
		case <-ctx.Done():
			return
		}
	}
}

func (node *Node) cancelSubscriptions() {
	if node.syncer.CancelChainSync != nil {
		node.syncer.CancelChainSync()
	}

	if node.syncer.BlockSub != nil {
		node.syncer.BlockSub.Cancel()
		node.syncer.BlockSub = nil
	}

	if node.Messaging.MessageSub != nil {
		node.Messaging.MessageSub.Cancel()
		node.Messaging.MessageSub = nil
	}
}

// Stop initiates the shutdown of the node.
func (node *Node) Stop(ctx context.Context) {
	node.chain.ChainReader.HeadEvents().Unsub(node.chain.HeaviestTipSetCh)
	node.StopMining(ctx)

	node.cancelSubscriptions()
	node.chain.ChainReader.Stop()

	if node.SectorBuilder() != nil {
		if err := node.SectorBuilder().Close(); err != nil {
			fmt.Printf("error closing sector builder: %s\n", err)
		}
		node.SectorStorage.SectorBuilder = nil
	}

	if err := node.Host().Close(); err != nil {
		fmt.Printf("error closing host: %s\n", err)
	}

	if err := node.Repo.Close(); err != nil {
		fmt.Printf("error closing repo: %s\n", err)
	}

	node.Discovery.Stop()

	fmt.Println("stopping filecoin :(")
}

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
	_, err = node.PorcelainAPI.ActorGetStable(ctx, minerAddr)
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
		storageMiner, err := initStorageMinerForNode(ctx, node)
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
		return errors.Wrap(err, "failed to setup mining")
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
	miningCtx, node.BlockMining.CancelMining = context.WithCancel(context.Background())

	outCh, doneWg := node.BlockMining.MiningScheduler.Start(miningCtx)

	node.BlockMining.MiningDoneWg = doneWg
	node.BlockMining.AddNewlyMinedBlock = node.addNewlyMinedBlock
	node.BlockMining.MiningDoneWg.Add(1)
	go node.handleNewMiningOutput(miningCtx, outCh)

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
					workerAddr, err := node.PorcelainAPI.MinerGetWorkerAddress(miningCtx, minerAddr, node.chain.ChainReader.GetHead())
					if err != nil {
						log.Errorf("failed to get worker address %s", err)
						continue
					}

					// This call can fail due to, e.g. nonce collisions. Our miners existence depends on this.
					// We should deal with this, but MessageSendWithRetry is problematic.
					msgCid, _, err := node.PorcelainAPI.MessageSend(
						miningCtx,
						workerAddr,
						minerAddr,
						types.ZeroAttoFIL,
						gasPrice,
						gasUnits,
						miner.CommitSector,
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
						log.Errorf("scheduler received error from node.SectorStorage.SectorBuilder.SealAllStagedSectors (%s) - exiting", err.Error())
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

// initStorageMinerForNode initializes the storage miner.
func initStorageMinerForNode(ctx context.Context, node *Node) (*storage.Miner, error) {
	minerAddr, err := node.MiningAddress()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node's mining address")
	}

	ownerAddress, err := node.PorcelainAPI.MinerGetOwnerAddress(ctx, minerAddr)
	if err != nil {
		return nil, errors.Wrap(err, "no mining owner available, skipping storage miner setup")
	}

	sectorSize, err := node.PorcelainAPI.MinerGetSectorSize(ctx, minerAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch miner's sector size")
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
		return nil, errors.Wrap(err, "failed to instantiate storage miner")
	}

	return miner, nil
}

// StopMining stops mining on new blocks.
func (node *Node) StopMining(ctx context.Context) {
	node.setIsMining(false)

	if node.BlockMining.CancelMining != nil {
		node.BlockMining.CancelMining()
	}

	if node.BlockMining.MiningDoneWg != nil {
		node.BlockMining.MiningDoneWg.Wait()
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
		node.chain.ChainReader,
		node.IsMining,
		mineDelay,
		node.SetupMining,
		node.StartMining,
		node.StopMining,
		node.GetMiningWorker)

	node.BlockMining.BlockMiningAPI = &blockMiningAPI

	// set up retrieval client and api
	retapi := retrieval.NewAPI(retrieval.NewClient(node.network.Host, node.PorcelainAPI))
	node.RetrievalProtocol.RetrievalAPI = &retapi

	// set up storage client and api
	smc := storage.NewClient(node.network.Host, node.PorcelainAPI)
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

		GetStateTree:   node.getStateTree,
		GetWeight:      node.getWeight,
		GetAncestors:   node.getAncestors,
		Election:       consensus.ElectionMachine{},
		TicketGen:      consensus.TicketMachine{},
		TipSetMetadata: node.chain.ChainReader,

		MessageSource: node.Messaging.Inbox.Pool(),
		MessageStore:  node.chain.MessageStore,
		Processor:     processor,
		Blockstore:    node.Blockstore.Blockstore,
		Clock:         node.Clock,
	}), nil
}

// getStateTree is the default GetStateTree function for the mining worker.
func (node *Node) getStateTree(ctx context.Context, ts block.TipSet) (state.Tree, error) {
	return node.chain.ChainReader.GetTipSetState(ctx, ts.Key())
}

// getWeight is the default GetWeight function for the mining worker.
func (node *Node) getWeight(ctx context.Context, ts block.TipSet) (uint64, error) {
	parent, err := ts.Parents()
	if err != nil {
		return uint64(0), err
	}
	// TODO handle genesis cid more gracefully
	if parent.Len() == 0 {
		return node.syncer.ChainSelector.Weight(ctx, ts, cid.Undef)
	}
	root, err := node.chain.ChainReader.GetTipSetStateRoot(parent)
	if err != nil {
		return uint64(0), err
	}
	return node.syncer.ChainSelector.Weight(ctx, ts, root)
}

// getAncestors is the default GetAncestors function for the mining worker.
func (node *Node) getAncestors(ctx context.Context, ts block.TipSet, newBlockHeight *types.BlockHeight) ([]block.TipSet, error) {
	ancestorHeight := newBlockHeight.Sub(types.NewBlockHeight(uint64(consensus.AncestorRoundsNeeded)))
	return chain.GetRecentAncestors(ctx, ts, node.chain.ChainReader, ancestorHeight)
}

// -- Accessors

// Host returns the nodes host.
func (node *Node) Host() host.Host {
	return node.network.Host
}

// SectorBuilder returns the nodes sectorBuilder.
func (node *Node) SectorBuilder() sectorbuilder.SectorBuilder {
	return node.SectorStorage.SectorBuilder
}

// BlockService returns the nodes blockservice.
func (node *Node) BlockService() bserv.BlockService {
	return node.Blockservice.Blockservice
}

// CborStore returns the nodes cborStore.
func (node *Node) CborStore() *hamt.CborIpldStore {
	return node.Blockstore.CborStore
}

// IsMining returns a boolean indicating whether the node is mining blocks.
func (node *Node) IsMining() bool {
	node.BlockMining.Mining.Lock()
	defer node.BlockMining.Mining.Unlock()
	return node.BlockMining.Mining.IsMining
}

// Chain returns the chain submodule.
func (node *Node) Chain() submodule.ChainSubmodule {
	return node.chain
}

// Syncer returns the syncer submodule.
func (node *Node) Syncer() submodule.SyncerSubmodule {
	return node.syncer
}

// Network returns the network submodule.
func (node *Node) Network() submodule.NetworkSubmodule {
	return node.network
}
