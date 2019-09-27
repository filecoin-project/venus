package node

import (
	"github.com/filecoin-project/go-filecoin/message"
	"github.com/filecoin-project/go-filecoin/net/pubsub"
)

// MessagingSubmodule enhances the `Node` with internal messaging capabilities.
type MessagingSubmodule struct {
	// Incoming messages for block mining.
	Inbox *message.Inbox

	// Messages sent and not yet mined.
	Outbox *message.Outbox

	// Network Fields
	MessageSub pubsub.Subscription
}
