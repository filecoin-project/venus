package api

import (
	"context"

	"github.com/filecoin-project/go-filecoin/types"
)

// Mining is the interface that defines methods to manage mining operations.
type Mining interface {
	Once(ctx context.Context) (*types.Block, error)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
