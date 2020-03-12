package submodule

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/journal"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net/pubsub"
	"github.com/pkg/errors"
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

	MsgPool   *message.Pool
	MsgSigVal *consensus.MessageSignatureValidator
}

type messagingConfig interface {
	Journal() journal.Journal
}

type messagingRepo interface {
	Config() *config.Config
}

// NewMessagingSubmodule creates a new discovery submodule.
func NewMessagingSubmodule(ctx context.Context, config messagingConfig, repo messagingRepo, network *NetworkSubmodule, chain *ChainSubmodule, wallet *WalletSubmodule) (MessagingSubmodule, error) {
	msgSyntaxValidator := consensus.NewMessageSyntaxValidator()
	msgSignatureValidator := consensus.NewMessageSignatureValidator(chain.State)
	msgPool := message.NewPool(repo.Config().Mpool, msgSyntaxValidator)
	inbox := message.NewInbox(msgPool, message.InboxMaxAgeTipsets, chain.ChainReader, chain.MessageStore)

	// setup messaging topic.
	// register block validation on pubsub
	mtv := net.NewMessageTopicValidator(msgSyntaxValidator, msgSignatureValidator)
	if err := network.pubsub.RegisterTopicValidator(mtv.Topic(network.NetworkName), mtv.Validator(), mtv.Opts()...); err != nil {
		return MessagingSubmodule{}, errors.Wrap(err, "failed to register message validator")
	}
	topic, err := network.pubsub.Join(net.MessageTopic(network.NetworkName))
	if err != nil {
		return MessagingSubmodule{}, err
	}

	msgQueue := message.NewQueue()
	outboxPolicy := message.NewMessageQueuePolicy(chain.MessageStore, message.OutboxMaxAgeRounds)
	msgPublisher := message.NewDefaultPublisher(pubsub.NewTopic(topic), msgPool)
	signer := wallet.Wallet
	outbox := message.NewOutbox(signer, msgSyntaxValidator, msgQueue, msgPublisher, outboxPolicy, chain.ChainReader, chain.State, config.Journal().Topic("outbox"))

	return MessagingSubmodule{
		Inbox:        inbox,
		Outbox:       outbox,
		MessageTopic: pubsub.NewTopic(topic),
		// MessageSub: nil,
		MsgPool:   msgPool,
		MsgSigVal: msgSignatureValidator,
	}, nil
}
