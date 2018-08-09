package api

import (
	"context"

	"github.com/filecoin-project/go-filecoin/types"
)

type Mpool interface {
	View(ctx context.Context, messageCount uint) ([]*types.SignedMessage, error)
}
