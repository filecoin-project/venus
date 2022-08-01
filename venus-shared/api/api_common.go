package api

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type Version interface {
	// Version provides information about API provider
	Version(ctx context.Context) (types.Version, error) //perm:read
}
