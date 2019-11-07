package submodule

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/journal"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net/pubsub"
)

// MessagingSubmodule enhances the `Node` with internal messaging capabilities.
type MessagingSubmodule struct {
	// Incoming messages for block mining.
	Inbox *message.Inbox

	// Messages sent and not yet mined.
	Outbox *message.Outbox

	// Network Fields
	MessageTopic *pubsub.Topic
	MessageSub   pubsub.Subscription

	MsgPool *message.Pool
}

type messagingConfig interface {
	Journal() journal.Journal
}

type messagingRepo interface {
	Config() *config.Config
}

// NewMessagingSubmodule creates a new discovery submodule.
func NewMessagingSubmodule(ctx context.Context, config messagingConfig, repo messagingRepo, network *NetworkSubmodule, chain *ChainSubmodule, wallet *WalletSubmodule) (MessagingSubmodule, error) {
	msgPool := message.NewPool(repo.Config().Mpool, consensus.NewIngestionValidator(chain.State, repo.Config().Mpool))
	inbox := message.NewInbox(msgPool, message.InboxMaxAgeTipsets, chain.ChainReader, chain.MessageStore)

	// setup messaging topic.
	topic, err := network.pubsub.Join(net.MessageTopic(network.NetworkName))
	if err != nil {
		return MessagingSubmodule{}, err
	}

	msgQueue := message.NewQueue()
	outboxPolicy := message.NewMessageQueuePolicy(chain.MessageStore, message.OutboxMaxAgeRounds)
	msgPublisher := message.NewDefaultPublisher(pubsub.NewTopic(topic), msgPool)
	outbox := message.NewOutbox(wallet.Wallet, consensus.NewOutboundMessageValidator(), msgQueue, msgPublisher, outboxPolicy, chain.ChainReader, chain.State, config.Journal().Topic("outbox"))

	return MessagingSubmodule{
		Inbox:        inbox,
		Outbox:       outbox,
		MessageTopic: pubsub.NewTopic(topic),
		// MessageSub: nil,
		MsgPool: msgPool,
	}, nil
}
