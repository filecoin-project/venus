package v1

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/api"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type ICommon interface {
	api.Version

	NodeStatus(ctx context.Context, inclChainStatus bool) (types.NodeStatus, error) //perm:read
}
