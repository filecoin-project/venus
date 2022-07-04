package syncer

import (
	"bytes"
	"context"
	"reflect"
	"runtime"
	"time"

	chain2 "github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/statemanger"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	cbor "github.com/ipfs/go-ipld-cbor"

	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/venus/app/submodule/blockstore"
	"github.com/filecoin-project/venus/app/submodule/discovery"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/chainsync"
	"github.com/filecoin-project/venus/pkg/chainsync/slashfilter"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/net/blocksub"
	"github.com/filecoin-project/venus/pkg/net/pubsub"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-blockservice"
)

var log = logging.Logger("sync.module") // nolint: deadcode

// SyncerSubmodule enhances the node with chain syncing capabilities
type SyncerSubmodule struct { //nolint
	BlockstoreModule   *blockstore.BlockstoreSubmodule
	ChainModule        *chain2.ChainSubmodule
	NetworkModule      *network.NetworkSubmodule
	DiscoverySubmodule *discovery.DiscoverySubmodule

	// todo: use the 'Topic' and 'Subscription' defined in
	//  "github.com/libp2p/go-libp2p-pubsub" replace which defined in
	//  'venus/pkg/net/pubsub/topic.go'
	BlockTopic       *pubsub.Topic
	BlockSub         pubsub.Subscription
	ChainSelector    nodeChainSelector
	Stmgr            *statemanger.Stmgr
	ChainSyncManager *chainsync.Manager
	Drand            beacon.Schedule
	SyncProvider     ChainSyncProvider
	SlashFilter      slashfilter.ISlashFilter
	BlockValidator   *consensus.BlockValidator

	// cancelChainSync cancels the context for chain sync subscriptions and handlers.
	CancelChainSync context.CancelFunc
}

type syncerConfig interface {
	GenesisCid() cid.Cid
	BlockTime() time.Duration
	ChainClock() clock.ChainEpochClock
	Repo() repo.Repo
	Verifier() ffiwrapper.Verifier
}

type nodeChainSelector interface {
	Weight(context.Context, *types.TipSet) (fbig.Int, error)
	IsHeavier(ctx context.Context, a, b *types.TipSet) (bool, error)
}

// NewSyncerSubmodule creates a new chain submodule.
func NewSyncerSubmodule(ctx context.Context,
	config syncerConfig,
	blockstore *blockstore.BlockstoreSubmodule,
	network *network.NetworkSubmodule,
	discovery *discovery.DiscoverySubmodule,
	chn *chain2.ChainSubmodule,
	circulatingSupplyCalculator chain.ICirculatingSupplyCalcualtor,
) (*SyncerSubmodule, error) {
	// setup validation
	gasPriceSchedule := gas.NewPricesSchedule(config.Repo().Config().NetworkParams.ForkUpgradeParam)

	tickets := consensus.NewTicketMachine(chn.ChainReader)
	cborStore := cbor.NewCborStore(config.Repo().Datastore())
	stateViewer := consensus.AsDefaultStateViewer(state.NewViewer(cborStore))
	nodeChainSelector := consensus.NewChainSelector(cborStore, &stateViewer)

	blkValid := consensus.NewBlockValidator(tickets,
		blockstore.Blockstore,
		chn.MessageStore,
		chn.Drand,
		cborStore,
		config.Verifier(),
		&stateViewer,
		chn.ChainReader,
		nodeChainSelector,
		chn.Fork,
		config.Repo().Config().NetworkParams,
		gasPriceSchedule)

	// register block validation on pubsub
	btv := blocksub.NewBlockTopicValidator(blkValid)
	if err := network.Pubsub.RegisterTopicValidator(btv.Topic(network.NetworkName), btv.Validator(), btv.Opts()...); err != nil {
		return nil, errors.Wrap(err, "failed to register block validator")
	}

	rnd := chn.API()
	nodeConsensus := consensus.NewExpected(cborStore,
		blockstore.Blockstore,
		chn.ChainReader,
		rnd,
		chn.MessageStore,
		chn.Fork,
		gasPriceSchedule,
		blkValid,
		chn.SystemCall,
		circulatingSupplyCalculator,
	)

	stmgr := statemanger.NewStateManger(chn.ChainReader, nodeConsensus, rnd,
		chn.Fork, gasPriceSchedule, chn.SystemCall)

	blkValid.Stmgr = stmgr
	chn.Stmgr = stmgr
	chn.Waiter.Stmgr = stmgr

	chainSyncManager, err := chainsync.NewManager(stmgr, blkValid, chn, nodeChainSelector,
		blockstore.Blockstore, discovery.ExchangeClient, config.ChainClock(), chn.Fork)
	if err != nil {
		return nil, err
	}

	discovery.PeerDiscoveryCallbacks = append(discovery.PeerDiscoveryCallbacks, func(ci *types.ChainInfo) {
		err := chainSyncManager.BlockProposer().SendHello(ci)
		if err != nil {
			log.Errorf("error receiving chain info from hello %s: %s", ci, err)
			return
		}
	})

	var (
		slashFilter slashfilter.ISlashFilter
	)
	if config.Repo().Config().SlashFilterDs.Type == "local" {
		slashFilter = slashfilter.NewLocalSlashFilter(config.Repo().ChainDatastore())
	} else {
		slashFilter, err = slashfilter.NewMysqlSlashFilter(config.Repo().Config().SlashFilterDs.MySQL)
		if err != nil {
			return nil, err
		}
	}

	return &SyncerSubmodule{
		Stmgr:              stmgr,
		BlockstoreModule:   blockstore,
		ChainModule:        chn,
		NetworkModule:      network,
		DiscoverySubmodule: discovery,
		SlashFilter:        slashFilter,
		ChainSelector:      nodeChainSelector,
		ChainSyncManager:   &chainSyncManager,
		Drand:              chn.Drand,
		SyncProvider:       *NewChainSyncProvider(&chainSyncManager),
		BlockValidator:     blkValid,
	}, nil
}

func (syncer *SyncerSubmodule) handleIncomingBlocks(ctx context.Context, msg pubsub.Message) error {
	sender := msg.GetSender()
	source := msg.GetSource()
	// ignore messages from self
	if sender == syncer.NetworkModule.Host.ID() || source == syncer.NetworkModule.Host.ID() {
		return nil
	}

	ctx, span := trace.StartSpan(ctx, "Node.handleIncomingBlocks")

	var bm types.BlockMsg
	err := bm.UnmarshalCBOR(bytes.NewReader(msg.GetData()))
	if err != nil {
		return errors.Wrapf(err, "failed to decode blocksub payload from source: %s, sender: %s", source, sender)
	}

	header := bm.Header
	span.AddAttributes(trace.StringAttribute("block", header.Cid().String()))

	log.Infof("Received new block %s height %d from peer %s", header.Cid(), header.Height, sender)

	_, err = syncer.ChainModule.ChainReader.PutObject(ctx, bm.Header)
	if err != nil {
		log.Errorf("failed to save block %s", err)
	}
	go func() {
		start := time.Now()

		defer func() {
			if cost := time.Since(start); cost > slowFetchMessageDuration {
				log.Warnf("incoming new block(%d, %s), slow fetch messages, cost time = %.4f(seconds)",
					bm.Header.Height, bm.Header.Cid().String(), cost.Seconds())
			}
			log.Infof("incoming new block(%d, %s), cost time = %.4f(seconds)",
				bm.Header.Height, bm.Header.Cid().String(), time.Since(start).Seconds())
		}()

		if delay := time.Since(time.Unix(int64(bm.Header.Timestamp), 0)); delay > incomeBlockLargeDelayDuration {
			log.Warnf("received block(%d, %s) with large delay : %s",
				bm.Header.Height, bm.Header.Cid(), delay.String())
		}

		blkSvc := blockservice.New(syncer.NetworkModule.Blockstore, syncer.NetworkModule.Bitswap)

		if _, err := syncer.NetworkModule.FetchMessagesByCids(ctx, blkSvc, bm.BlsMessages); err != nil {
			log.Errorf("fetch block bls messages failed:%s", err.Error())
			return
		}
		if _, err := syncer.NetworkModule.FetchSignedMessagesByCids(ctx, blkSvc, bm.SecpkMessages); err != nil {
			log.Errorf("fetch block signed messages failed:%s", err.Error())
			return
		}

		syncer.NetworkModule.Host.ConnManager().TagPeer(sender, "new-block", 20)
		log.Infof("fetch message success at %s", bm.Header.Cid())

		ts, _ := types.NewTipSet([]*types.BlockHeader{header})
		chainInfo := types.NewChainInfo(source, sender, ts)

		if err = syncer.ChainSyncManager.BlockProposer().SendGossipBlock(chainInfo); err != nil {
			log.Errorf("failed to notify syncer of new block, block: %s", err)
		}

	}()
	return nil
}

// Start starts the syncer submodule for a node.
func (syncer *SyncerSubmodule) Start(ctx context.Context) error {
	// setup topic
	topic, err := syncer.NetworkModule.Pubsub.Join(blocksub.Topic(syncer.NetworkModule.NetworkName))
	if err != nil {
		return err
	}
	syncer.BlockTopic = pubsub.NewTopic(topic)

	syncer.BlockSub, err = syncer.BlockTopic.Subscribe()
	if err != nil {
		return errors.Wrapf(err, "failed to subscribe block topic")
	}

	//process incoming blocks
	go func() {
		for {
			received, err := syncer.BlockSub.Next(ctx)
			if err != nil {
				if ctx.Err() != context.Canceled {
					log.Errorf("error reading message from topic %s: %s", syncer.BlockSub.Topic(), err)
				}
				return
			}

			if err := syncer.handleIncomingBlocks(ctx, received); err != nil {
				handlerName := runtime.FuncForPC(reflect.ValueOf(syncer.handleIncomingBlocks).Pointer()).Name()
				if err != context.Canceled {
					log.Debugf("error in handler %s for topic %s: %s", handlerName, syncer.BlockSub.Topic(), err)
				}
			}
		}
	}()

	err = syncer.ChainModule.Start(ctx)
	if err != nil {
		return err
	}

	return syncer.ChainSyncManager.Start(ctx)
}

func (syncer *SyncerSubmodule) Stop(ctx context.Context) {
	if syncer.CancelChainSync != nil {
		syncer.CancelChainSync()
	}
	if syncer.BlockSub != nil {
		syncer.BlockSub.Cancel()
	}
	if syncer.Stmgr != nil {
		syncer.Stmgr.Close(ctx)
	}
}

//API create a new sync api implement
func (syncer *SyncerSubmodule) API() v1api.ISyncer {
	return &syncerAPI{syncer: syncer}
}

func (syncer *SyncerSubmodule) V0API() v0api.ISyncer {
	return &syncerAPI{syncer: syncer}
}
