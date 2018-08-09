package api

import (
	"context"

	"github.com/filecoin-project/go-filecoin/types"
)

type Mining interface {
	Once(ctx context.Context) (*types.Block, error)
	Start() error
	Stop() error
}
