package node

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"runtime"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-sectorbuilder"
	"github.com/filecoin-project/go-sectorbuilder/fs"
	"github.com/filecoin-project/specs-actors/actors/abi"
	fbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/internal/submodule"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paths"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics"
	"github.com/filecoin-project/go-filecoin/internal/pkg/mining"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net/pubsub"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	mining_protocol "github.com/filecoin-project/go-filecoin/internal/pkg/protocol/mining"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/version"
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

	// ChainClock is a chainClock used by the node for chain epoch.
	ChainClock clock.ChainEpochClock

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
	StorageMining *submodule.StorageMiningSubmodule

	//
	// Supporting services
	//

	Wallet            submodule.WalletSubmodule
	Messaging         submodule.MessagingSubmodule
	StorageNetworking submodule.StorageNetworkingSubmodule
	ProofVerification submodule.ProofVerificationSubmodule

	//
	// Protocols
	//

	VersionTable      *version.ProtocolVersionTable
	StorageProtocol   *submodule.StorageProtocolSubmodule
	RetrievalProtocol *submodule.RetrievalProtocolSubmodule
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
		if err := node.setupStorageMining(ctx); err != nil {
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

	// DRAGONS: uncomment when we have retrieval market integration
	//node.RetrievalProtocol.RetrievalProvider = retrieval.NewMiner()

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

		// Wire up syncing and possible mining
		go node.doMiningPause(syncCtx)
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

			if node.StorageMining != nil {
				if err := node.StorageMining.HandleNewHead(ctx, newHead); err != nil {
					log.Error(err)
				}
			}

			if err := handler.HandleNewHead(ctx, newHead); err != nil {
				log.Error(err)
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

	if node.StorageMining != nil {
		if err := node.StorageMining.Stop(ctx); err != nil {
			fmt.Printf("error stopping storage miner: %s\n", err)
		}
		node.StorageMining = nil
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

func (node *Node) addMinedBlockSynchronous(ctx context.Context, b *block.Block) error {
	wait := node.syncer.ChainSyncManager.BlockProposer().WaiterForTarget(block.NewTipSetKey(b.Cid()))
	err := node.AddNewBlock(ctx, b)
	if err != nil {
		return err
	}
	err = wait()
	return err
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

// SetupMining initializes all the functionality the node needs to start mining.
// This method is idempotent.
func (node *Node) SetupMining(ctx context.Context) error {
	// ensure we have a miner actor before we even consider mining
	minerAddr, err := node.MiningAddress()
	if err != nil {
		return errors.Wrap(err, "failed to get mining address")
	}
	head := node.PorcelainAPI.ChainHeadKey()
	_, err = node.PorcelainAPI.MinerGetStatus(ctx, minerAddr, head)
	if err != nil {
		return errors.Wrap(err, "failed to get miner actor")
	}

	// ensure we've got our storage mining submodule configured
	if node.StorageMining == nil {
		if err := node.setupStorageMining(ctx); err != nil {
			return err
		}
	}

	if node.RetrievalProtocol == nil {
		if err := node.setupRetrievalMining(ctx); err != nil {
			return err
		}
	}
	// ensure we have a mining worker
	if node.BlockMining.MiningWorker == nil {
		if node.BlockMining.MiningWorker, err = node.CreateMiningWorker(ctx); err != nil {
			return err
		}
	}

	return nil
}

func registeredProofsFromSectorSize(ss abi.SectorSize) (registeredSealProof abi.RegisteredProof, registeredPoStProof abi.RegisteredProof, err error) {
	switch ss {
	case constants.DevSectorSize:
		return constants.DevRegisteredPoStProof, constants.DevRegisteredSealProof, nil
	case constants.ThirtyTwoGiBSectorSize:
		return abi.RegisteredProof_StackedDRG32GiBPoSt, abi.RegisteredProof_StackedDRG32GiBSeal, nil
	case constants.EightMiBSectorSize:
		return abi.RegisteredProof_StackedDRG8MiBPoSt, abi.RegisteredProof_StackedDRG8MiBSeal, nil
	case constants.FiveHundredTwelveMiBSectorSize:
		return abi.RegisteredProof_StackedDRG512MiBPoSt, abi.RegisteredProof_StackedDRG512MiBSeal, nil
	default:
		return 0, 0, errors.Errorf("unsupported sector size %d", ss)
	}
}

func (node *Node) setupStorageMining(ctx context.Context) error {
	if node.StorageMining != nil {
		return errors.New("storage mining submodule has already been initialized")
	}

	minerAddr, err := node.MiningAddress()
	if err != nil {
		return err
	}

	head := node.Chain().ChainReader.GetHead()
	status, err := node.PorcelainAPI.MinerGetStatus(ctx, minerAddr, head)
	if err != nil {
		return err
	}

	repoPath, err := node.Repo.Path()
	if err != nil {
		return err
	}

	sectorDir, err := paths.GetSectorPath(node.Repo.Config().SectorBase.RootDir, repoPath)
	if err != nil {
		return err
	}

	postProofType, sealProofType, err := registeredProofsFromSectorSize(status.SectorSize)
	if err != nil {
		return err
	}

	sectorBuilder, err := sectorbuilder.New(&sectorbuilder.Config{
		PoStProofType: postProofType,
		SealProofType: sealProofType,
		Miner:         minerAddr,
		WorkerThreads: 1,
		Paths: []fs.PathConfig{
			{
				Path:   sectorDir,
				Cache:  false,
				Weight: 1,
			},
		},
	}, namespace.Wrap(node.Repo.Datastore(), ds.NewKey("/sectorbuilder")))
	if err != nil {
		return err
	}

	cborStore := node.Blockstore.CborStore

	waiter := msg.NewWaiter(node.chain.ChainReader, node.chain.MessageStore, node.Blockstore.Blockstore, cborStore)

	// TODO: rework these modules so they can be at least partially constructed during the building phase #3738
	stateViewer := state.NewViewer(cborStore)

	node.StorageMining, err = submodule.NewStorageMiningSubmodule(minerAddr, node.Repo.Datastore(),
		sectorBuilder, &node.chain, &node.Messaging, waiter, &node.Wallet, stateViewer, node.BlockMining.PoStGenerator)
	if err != nil {
		return err
	}

	node.StorageProtocol, err = submodule.NewStorageProtocolSubmodule(
		ctx,
		minerAddr,
		address.Undef, // TODO: This is for setting up mining, we need to pass the client address in if this is going to be a storage client also
		&node.chain,
		&node.Messaging,
		waiter,
		node.StorageMining.PieceManager,
		node.Wallet.Wallet,
		node.Host(),
		node.Repo.Datastore(),
		node.Blockstore.Blockstore,
		node.network.GraphExchange,
		repoPath,
		sectorBuilder.SealProofType(),
		stateViewer,
	)
	if err != nil {
		return errors.Wrap(err, "error initializing storage protocol")
	}

	return nil
}

func (node *Node) setupRetrievalMining(ctx context.Context) error {
	providerAddr, err := node.MiningAddress()
	if err != nil {
		return errors.Wrap(err, "failed to get mining address")
	}
	rp, err := submodule.NewRetrievalProtocolSubmodule(
		node.Blockstore.Blockstore,
		node.Repo.Datastore(),
		node.chain.State,
		node.Host(),
		providerAddr,
		node.Wallet.Wallet,
		nil, // TODO: payment channel manager API, in follow-up
		node.PieceManager(),
	)
	if err != nil {
		return errors.Wrap(err, "failed to build node.RetrievalProtocol")
	}
	node.RetrievalProtocol = rp
	return nil
}

func (node *Node) doMiningPause(ctx context.Context) {
	// doMiningPause receives state transition signals from the syncer
	// dispatcher allowing syncing to make progress.
	//
	// When mining, the node passes these signals along to the scheduler
	// pausing and continuing mining based on syncer state.
	catchupCh := node.Syncer().ChainSyncManager.TransitionChannel()
	for {
		select {
		case <-ctx.Done():
			return
		case toCatchup, ok := <-catchupCh:
			if !ok {
				return
			}
			if node.BlockMining.MiningScheduler == nil {
				// drop syncer transition signals if not mining
				continue
			}
			if toCatchup {
				node.BlockMining.MiningScheduler.Pause()
			} else {
				node.BlockMining.MiningScheduler.Continue()
			}
		}
	}
}

// StartMining causes the node to start feeding blocks to the mining worker and initializes
// the StorageMining for the mining address.
func (node *Node) StartMining(ctx context.Context) error {
	if node.IsMining() {
		return errors.New("Node is already mining")
	}

	err := node.SetupMining(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to setup mining")
	}

	if node.BlockMining.MiningScheduler == nil {
		node.BlockMining.MiningScheduler = mining.NewScheduler(node.BlockMining.MiningWorker, node.PorcelainAPI.ChainHead, node.ChainClock)
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

	if err := node.StorageMining.Start(ctx); err != nil {
		fmt.Printf("error starting storage miner: %s\n", err)
	}

	if err := node.StorageProtocol.StorageProvider.Start(ctx); err != nil {
		fmt.Printf("error starting storage provider: %s\n", err)
	}

	// TODO: Retrieval Market Integration
	//if err := node.RetrievalProtocol.RetrievalProvider.Start(); err != nil {
	//	fmt.Printf("error starting retrieval provider: %s\n", err)
	//}

	node.setIsMining(true)

	return nil
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

	if node.StorageMining != nil {
		err := node.StorageMining.Stop(ctx)
		if err != nil {
			log.Warn("Error stopping storage miner", err)
		}
	}
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
			if err != context.Canceled {
				log.Errorf("error in handler %s for topic %s: %s", handlerName, sub.Topic(), err)
			}
		}
	}
}

// setupProtocols creates protocol clients and miners, then sets the node's APIs
// for each
func (node *Node) setupProtocols() error {
	blockMiningAPI := mining_protocol.New(
		node.MiningAddress,
		node.addMinedBlockSynchronous,
		node.chain.ChainReader,
		node.IsMining,
		node.SetupMining,
		node.StartMining,
		node.StopMining,
		node.GetMiningWorker,
		node.ChainClock,
	)

	node.BlockMining.BlockMiningAPI = &blockMiningAPI
	return nil
}

// GetMiningWorker ensures mining is setup and then returns the worker
func (node *Node) GetMiningWorker(ctx context.Context) (*mining.DefaultWorker, error) {
	if err := node.SetupMining(ctx); err != nil {
		return nil, err
	}
	return node.BlockMining.MiningWorker, nil
}

// CreateMiningWorker creates a mining.Worker for the node using the configured
// getStateTree, getWeight, and getAncestors functions for the node
func (node *Node) CreateMiningWorker(ctx context.Context) (*mining.DefaultWorker, error) {
	minerAddr, err := node.MiningAddress()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get mining address")
	}

	head := node.PorcelainAPI.ChainHeadKey()
	minerStatus, err := node.PorcelainAPI.MinerGetStatus(ctx, minerAddr, head)
	if err != nil {
		log.Errorf("could not get owner address of miner actor")
		return nil, err
	}

	return mining.NewDefaultWorker(mining.WorkerParameters{
		API: node.PorcelainAPI,

		MinerAddr:      minerAddr,
		MinerOwnerAddr: minerStatus.OwnerAddress,
		WorkerSigner:   node.Wallet.Wallet,

		GetStateTree:   node.chain.ChainReader.GetTipSetState,
		GetWeight:      node.getWeight,
		Election:       consensus.NewElectionMachine(node.PorcelainAPI),
		TicketGen:      consensus.NewTicketMachine(node.PorcelainAPI),
		TipSetMetadata: node.chain.ChainReader,

		MessageSource:    node.Messaging.Inbox.Pool(),
		MessageStore:     node.chain.MessageStore,
		MessageQualifier: consensus.NewMessagePenaltyChecker(node.Chain().State),
		Blockstore:       node.Blockstore.Blockstore,
		Clock:            node.ChainClock,
		Poster:           node.StorageMining.PoStGenerator,
	}), nil
}

// getWeight is the default GetWeight function for the mining worker.
func (node *Node) getWeight(ctx context.Context, ts block.TipSet) (fbig.Int, error) {
	parent, err := ts.Parents()
	if err != nil {
		return fbig.Zero(), err
	}
	var baseStRoot cid.Cid
	if parent.Empty() {
		// use genesis state as parent state of genesis block
		baseStRoot, err = node.chain.ChainReader.GetTipSetStateRoot(ts.Key())
	} else {
		baseStRoot, err = node.chain.ChainReader.GetTipSetStateRoot(parent)
	}
	if err != nil {
		return fbig.Zero(), err
	}
	return node.syncer.ChainSelector.Weight(ctx, ts, baseStRoot)
}

// -- Accessors

// Host returns the nodes host.
func (node *Node) Host() host.Host {
	return node.network.Host
}

// PieceManager returns the node's PieceManager.
func (node *Node) PieceManager() piecemanager.PieceManager {
	return node.StorageMining.PieceManager
}

// BlockService returns the nodes blockservice.
func (node *Node) BlockService() bserv.BlockService {
	return node.Blockservice.Blockservice
}

// CborStore returns the nodes cborStore.
func (node *Node) CborStore() *cborutil.IpldStore {
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
