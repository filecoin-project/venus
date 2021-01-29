package syncer

import (
	"bytes"
	"context"
	"go.opencensus.io/trace"
	"reflect"
	"runtime"
	"time"

	"github.com/filecoin-project/venus/pkg/chainsync/slashfilter"
	"github.com/filecoin-project/venus/pkg/vm/gas"

	"github.com/filecoin-project/venus/pkg/repo"

	"github.com/filecoin-project/venus/app/submodule/blockstore"
	chain2 "github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/discovery"
	"github.com/filecoin-project/venus/app/submodule/network"

	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/chainsync"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/net/blocksub"
	"github.com/filecoin-project/venus/pkg/net/pubsub"
	"github.com/filecoin-project/venus/pkg/slashing"
	"github.com/filecoin-project/venus/pkg/state"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("sync.module") // nolint: deadcode

// SyncerSubmodule enhances the node with chain syncing capabilities
type SyncerSubmodule struct { //nolint
	BlockstoreModule   *blockstore.BlockstoreSubmodule
	ChainModule        *chain2.ChainSubmodule
	NetworkModule      *network.NetworkSubmodule
	DiscoverySubmodule *discovery.DiscoverySubmodule

	BlockTopic       *pubsub.Topic
	BlockSub         pubsub.Subscription
	ChainSelector    nodeChainSelector
	Consensus        consensus.Protocol
	FaultDetector    slashing.ConsensusFaultDetector
	ChainSyncManager *chainsync.Manager
	Drand            beacon.Schedule
	SyncProvider     ChainSyncProvider
	SlashFilter      *slashfilter.SlashFilter
	BlockValidator   *consensus.BlockValidator
	// cancelChainSync cancels the context for chain sync subscriptions and handlers.
	CancelChainSync context.CancelFunc
	// faultCh receives detected consensus faults
	faultCh chan slashing.ConsensusFault
}

type syncerConfig interface {
	GenesisCid() cid.Cid
	BlockTime() time.Duration
	ChainClock() clock.ChainEpochClock
	Repo() repo.Repo
}

type nodeChainSelector interface {
	Weight(context.Context, *block.TipSet) (fbig.Int, error)
	IsHeavier(ctx context.Context, a, b *block.TipSet) (bool, error)
}

// NewSyncerSubmodule creates a new chain submodule.
func NewSyncerSubmodule(ctx context.Context,
	config syncerConfig,
	blockstore *blockstore.BlockstoreSubmodule,
	network *network.NetworkSubmodule,
	discovery *discovery.DiscoverySubmodule,
	chn *chain2.ChainSubmodule,
	postVerifier consensus.ProofVerifier) (*SyncerSubmodule, error) {
	// setup validation
	gasPriceSchedule := gas.NewPricesSchedule(config.Repo().Config().NetworkParams.ForkUpgradeParam)

	genBlk, err := chn.ChainReader.GetGenesisBlock(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to locate genesis block during node build")
	}

	// set up consensus
	//	elections := consensus.NewElectionMachine(chn.state)
	sampler := chain.NewSampler(chn.ChainReader, genBlk.Ticket)
	tickets := consensus.NewTicketMachine(sampler, chn.ChainReader)
	stateViewer := consensus.AsDefaultStateViewer(state.NewViewer(blockstore.CborStore))
	nodeChainSelector := consensus.NewChainSelector(blockstore.CborStore, &stateViewer)

	blkValid := consensus.NewBlockValidator(tickets,
		blockstore.Blockstore,
		chn.MessageStore,
		chn.Drand,
		blockstore.CborStore,
		postVerifier,
		&stateViewer,
		chn.ChainReader,
		nodeChainSelector,
		chn.Fork,
		config.Repo().Config().NetworkParams,
		gasPriceSchedule,
	)
	// register block validation on pubsub
	btv := blocksub.NewBlockTopicValidator(blkValid)
	if err := network.Pubsub.RegisterTopicValidator(btv.Topic(network.NetworkName), btv.Validator(), btv.Opts()...); err != nil {
		return nil, errors.Wrap(err, "failed to register block validator")
	}
	nodeConsensus := consensus.NewExpected(blockstore.CborStore,
		blockstore.Blockstore,
		config.BlockTime(),
		chn.ChainReader,
		chn.State,
		chn.MessageStore,
		chn.Fork,
		config.Repo().Config().NetworkParams,
		gasPriceSchedule,
		postVerifier,
		blkValid,
	)

	faultCh := make(chan slashing.ConsensusFault)
	faultDetector := slashing.NewConsensusFaultDetector(faultCh)

	chainSyncManager, err := chainsync.NewManager(nodeConsensus, blkValid, nodeChainSelector, chn.ChainReader, chn.MessageStore, blockstore.Blockstore, discovery.ExchangeClient, config.ChainClock(), faultDetector, chn.Fork)
	if err != nil {
		return nil, err
	}

	discovery.PeerDiscoveryCallbacks = append(discovery.PeerDiscoveryCallbacks, func(ci *block.ChainInfo) {
		err := chainSyncManager.BlockProposer().SendHello(ci)
		if err != nil {
			log.Errorf("error receiving chain info from hello %s: %s", ci, err)
			return
		}
	})

	return &SyncerSubmodule{
		BlockstoreModule:   blockstore,
		ChainModule:        chn,
		NetworkModule:      network,
		DiscoverySubmodule: discovery,
		SlashFilter:        slashfilter.New(config.Repo().ChainDatastore()),
		Consensus:          nodeConsensus,
		ChainSelector:      nodeChainSelector,
		ChainSyncManager:   &chainSyncManager,
		Drand:              chn.Drand,
		SyncProvider:       *NewChainSyncProvider(&chainSyncManager),
		faultCh:            faultCh,
		BlockValidator:     blkValid,
	}, nil
}

func (syncer *SyncerSubmodule) handleIncommingBlocks(ctx context.Context, msg pubsub.Message) error {
	sender := msg.GetSender()
	source := msg.GetSource()
	// ignore messages from self
	if sender == syncer.NetworkModule.Host.ID() || source == syncer.NetworkModule.Host.ID() {
		return nil
	}

	ctx, span := trace.StartSpan(ctx, "Node.handleIncommingBlocks")

	var bm block.BlockMsg
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
		_, err = syncer.NetworkModule.FetchMessagesByCids(ctx, bm.BlsMessages)
		if err != nil {
			log.Errorf("failed to fetch all bls messages for block received over pubusb: %s; source: %s", err, source)
			return
		}

		_, err = syncer.NetworkModule.FetchSignedMessagesByCids(ctx, bm.SecpkMessages)
		if err != nil {
			log.Errorf("failed to fetch all secpk messages for block received over pubusb: %s; source: %s", err, source)
			return
		}

		syncer.NetworkModule.Host.ConnManager().TagPeer(sender, "new-block", 20)
		log.Infof("fetch message success at %s", bm.Header.Cid())
		ts, _ := block.NewTipSet(header)
		chainInfo := block.NewChainInfo(source, sender, ts)
		err = syncer.ChainSyncManager.BlockProposer().SendGossipBlock(chainInfo)
		if err != nil {
			log.Errorf("failed to notify syncer of new block, block: %s", err)
		}
	}()
	return nil
}

func (syncer *SyncerSubmodule) loadLocalFullTipset(ctx context.Context, tsk block.TipSetKey) (*block.FullTipSet, error) {
	ts, err := syncer.ChainModule.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, err
	}

	fts := &block.FullTipSet{}
	for _, b := range ts.Blocks() {
		smsgs, bmsgs, err := syncer.ChainModule.MessageStore.LoadMetaMessages(ctx, b.Messages)
		if err != nil {
			return nil, err
		}

		fb := &block.FullBlock{
			Header:       b,
			BLSMessages:  bmsgs,
			SECPMessages: smsgs,
		}
		fts.Blocks = append(fts.Blocks, fb)
	}

	return fts, nil
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

			if err := syncer.handleIncommingBlocks(ctx, received); err != nil {
				handlerName := runtime.FuncForPC(reflect.ValueOf(syncer.handleIncommingBlocks).Pointer()).Name()
				if err != context.Canceled {
					log.Debugf("error in handler %s for topic %s: %s", handlerName, syncer.BlockSub.Topic(), err)
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-syncer.faultCh:
				// TODO #3690 connect this up to a slasher that sends messages
				// to outbound queue to carry out penalization
			}
		}
	}()

	return syncer.ChainSyncManager.Start(ctx)
}

func (syncer *SyncerSubmodule) Stop(ctx context.Context) {
	if syncer.CancelChainSync != nil {
		syncer.CancelChainSync()
	}
	if syncer.BlockSub != nil {
		syncer.BlockSub.Cancel()
	}
}

func (syncer *SyncerSubmodule) API() *SyncerAPI {
	return &SyncerAPI{syncer: syncer}
}
