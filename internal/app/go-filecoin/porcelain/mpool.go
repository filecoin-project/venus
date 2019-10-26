package porcelain

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// The subset of plumbing used by MessagePoolWait
type mpwPlumbing interface {
	MessagePoolPending() []*types.SignedMessage
}

// MessagePoolWait waits until the message pool contains at least messageCount unmined messages.
func MessagePoolWait(ctx context.Context, plumbing mpwPlumbing, messageCount uint) ([]*types.SignedMessage, error) {
	pending := plumbing.MessagePoolPending()
	for len(pending) < int(messageCount) {
		// Poll pending again after subscribing in case a message arrived since.
		pending = plumbing.MessagePoolPending()
		time.Sleep(200 * time.Millisecond)
	}

	return pending, nil
}
