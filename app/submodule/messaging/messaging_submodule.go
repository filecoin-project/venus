package messaging

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/app/submodule/blockstore"
	chainModule "github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/messaging/msg"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/app/submodule/syncer"
	"github.com/filecoin-project/venus/app/submodule/wallet"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/journal"
	"github.com/filecoin-project/venus/pkg/message"
	"github.com/filecoin-project/venus/pkg/net/msgsub"
	"github.com/filecoin-project/venus/pkg/net/pubsub"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm/state"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"
)

var messagingLogger = logging.Logger("messaging")

// MessagingSubmodule enhances the `Node` with internal messaging capabilities.
type MessagingSubmodule struct { //nolint
	// Incoming messages for block mining.
	Inbox *message.Inbox

	// Messages sent and not yet mined.
	Outbox *message.Outbox

	// Wait for confirm message
	Waiter *msg.Waiter
	// Network Fields
	MessageTopic *pubsub.Topic
	MessageSub   pubsub.Subscription

	MsgPool   *message.Pool
	MsgSigVal *consensus.MessageSignatureValidator

	chainReader  chainReader
	messageStore *chain.MessageStore
}

type messagingConfig interface {
	Journal() journal.Journal
}

type messagingRepo interface {
	Config() *config.Config
}

type chainReader interface {
	chain.TipSetProvider
	GetHead() block.TipSetKey
	GetTipSetReceiptsRoot(block.TipSetKey) (cid.Cid, error)
	GetTipSetStateRoot(block.TipSetKey) (cid.Cid, error)
	SubHeadChanges(context.Context) chan []*chain.HeadChange
	SubscribeHeadChanges(chain.ReorgNotifee)
}
type stateReader interface {
	ResolveAddressAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (address.Address, error)
	GetActorAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (*types.Actor, error)
	GetTipSetState(context.Context, block.TipSetKey) (state.Tree, error)
}

// NewMessagingSubmodule creates a new discovery submodule.
func NewMessagingSubmodule(ctx context.Context,
	config messagingConfig,
	repo messagingRepo,
	network *network.NetworkSubmodule,
	chain *chainModule.ChainSubmodule,
	bsModule *blockstore.BlockstoreSubmodule,
	wallet *wallet.WalletSubmodule,
	syncer *syncer.SyncerSubmodule,
) (*MessagingSubmodule, error) {
	msgSyntaxValidator := consensus.NewMessageSyntaxValidator()
	msgSignatureValidator := consensus.NewMessageSignatureValidator(chain.State)

	msgPool := message.NewPool(repo.Config().Mpool, msgSyntaxValidator)
	inbox := message.NewInbox(msgPool, message.InboxMaxAgeTipsets, chain.ChainReader, chain.MessageStore)

	// setup messaging topic.
	// register block validation on pubsub
	mtv := msgsub.NewMessageTopicValidator(msgSyntaxValidator, msgSignatureValidator)
	if err := network.Pubsub.RegisterTopicValidator(mtv.Topic(network.NetworkName), mtv.Validator(), mtv.Opts()...); err != nil {
		return nil, errors.Wrap(err, "failed to register message validator")
	}

	topic, err := network.Pubsub.Join(msgsub.Topic(network.NetworkName))
	if err != nil {
		return nil, err
	}

	msgQueue := message.NewQueue()
	outboxPolicy := message.NewMessageQueuePolicy(chain.MessageStore, message.OutboxMaxAgeRounds, msgPool)
	msgPublisher := message.NewDefaultPublisher(pubsub.NewTopic(topic), msgPool)
	outbox := message.NewOutbox(wallet.Signer, msgSyntaxValidator, msgQueue, msgPublisher, outboxPolicy, chain.ChainReader, chain.State,
		config.Journal().Topic("outbox"), syncer.Consensus)

	combineChainReader := struct {
		stateReader
		chainReader
	}{
		chain.State,
		chain.ChainReader,
	}
	waiter := msg.NewWaiter(combineChainReader, chain.MessageStore, bsModule.Blockstore, bsModule.CborStore)
	return &MessagingSubmodule{
		Inbox:        inbox,
		Outbox:       outbox,
		MessageTopic: pubsub.NewTopic(topic),
		// MessageSub: nil,
		MsgPool:      msgPool,
		MsgSigVal:    msgSignatureValidator,
		chainReader:  chain.ChainReader,
		Waiter:       waiter,
		messageStore: chain.MessageStore,
	}, nil
}

func (messaging *MessagingSubmodule) Start(ctx context.Context) error {
	handler := message.NewHeadHandler(messaging.Inbox, messaging.Outbox, messaging.chainReader)

	messaging.chainReader.SubscribeHeadChanges(func(rev, app []*block.TipSet) error {
		if err := handler.HandleNewHead(ctx, rev, app); err != nil {
			messagingLogger.Error(err)
		}
		return nil
	})
	return nil
}

func (messaging *MessagingSubmodule) API() *MessagingAPI {
	return &MessagingAPI{messaging: messaging}
}
