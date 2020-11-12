package node

import (
	"context"
	"fmt"
	"reflect"
	"runtime"

	fbig "github.com/filecoin-project/go-state-types/big"
	bserv "github.com/ipfs/go-blockservice"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/internal/submodule"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/cborutil"
	"github.com/filecoin-project/venus/internal/pkg/chain"
	"github.com/filecoin-project/venus/internal/pkg/clock"
	"github.com/filecoin-project/venus/internal/pkg/message"
	"github.com/filecoin-project/venus/internal/pkg/metrics"
	"github.com/filecoin-project/venus/internal/pkg/net/pubsub"
	"github.com/filecoin-project/venus/internal/pkg/protocol/drand"
	"github.com/filecoin-project/venus/internal/pkg/repo"
	"github.com/filecoin-project/venus/internal/pkg/version"
)

var log = logging.Logger("node") // nolint: deadcode

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
	DrandAPI     *drand.API

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

	chain  submodule.ChainSubmodule
	syncer submodule.SyncerSubmodule

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

	VersionTable *version.ProtocolVersionTable
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

	var syncCtx context.Context
	syncCtx, node.syncer.CancelChainSync = context.WithCancel(context.Background())

	// Wire up propagation of new chain heads from the chain store to other components.
	head, err := node.PorcelainAPI.ChainHead()
	if err != nil {
		return errors.Wrap(err, "failed to get chain head")
	}
	go node.handleNewChainHeads(syncCtx, head)

	if !node.OfflineMode {
		// Start node discovery
		if err := node.Discovery.Start(node); err != nil {
			return err
		}

		// Subscribe to block pubsub topic to learn about new chain heads.
		node.syncer.BlockSub, err = node.pubsubscribe(syncCtx, node.syncer.BlockTopic, node.handleBlockSub)
		if err != nil {
			log.Error(err)
		}

		// Subscribe to the message pubsub topic to learn about messages to mine into blocks.
		node.Messaging.MessageSub, err = node.pubsubscribe(syncCtx, node.Messaging.MessageTopic, node.processMessage)
		if err != nil {
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

func (node *Node) handleNewChainHeads(ctx context.Context, firstHead *block.TipSet) {
	newHeadCh := node.chain.ChainReader.HeadEvents().Sub(chain.NewHeadTopic)
	defer log.Infof("new head handler exited")
	defer node.chain.ChainReader.HeadEvents().Unsub(newHeadCh)

	handler := message.NewHeadHandler(node.Messaging.Inbox, node.Messaging.Outbox, node.chain.ChainReader, firstHead)

	for {
		log.Debugf("waiting for new head")
		select {
		case ts, ok := <-newHeadCh:
			if !ok {
				log.Errorf("failed new head channel receive")
				return
			}
			newHead, ok := ts.(*block.TipSet)
			if !ok {
				log.Errorf("non-tipset published on heaviest tipset channel")
				continue
			}

			log.Debugf("message pool handling new head")
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
	node.cancelSubscriptions()
	node.chain.ChainReader.Stop()

	if err := node.Host().Close(); err != nil {
		fmt.Printf("error closing host: %s\n", err)
	}

	if err := node.Repo.Close(); err != nil {
		fmt.Printf("error closing repo: %s\n", err)
	}

	node.Discovery.Stop()

	fmt.Println("stopping filecoin :(")
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

// getWeight is the default GetWeight function for the mining worker.
func (node *Node) getWeight(ctx context.Context, ts *block.TipSet) (fbig.Int, error) {
	return node.syncer.ChainSelector.Weight(ctx, ts)
}

// -- Accessors

// Host returns the nodes host.
func (node *Node) Host() host.Host {
	return node.network.Host
}

// BlockService returns the nodes blockservice.
func (node *Node) BlockService() bserv.BlockService {
	return node.Blockservice.Blockservice
}

// CborStore returns the nodes cborStore.
func (node *Node) CborStore() *cborutil.IpldStore {
	return node.Blockstore.CborStore
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
