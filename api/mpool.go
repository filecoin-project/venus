package api

import (
	"context"

	"github.com/filecoin-project/go-filecoin/types"
)

// Mpool is the interface that defines methods to interact with the memory pool.
type Mpool interface {
	View(ctx context.Context, messageCount uint) ([]*types.SignedMessage, error)
}
