package v1

import (
	"context"
	"time"

	"github.com/filecoin-project/venus/venus-shared/api"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type ICommon interface {
	api.Version

	NodeStatus(ctx context.Context, inclChainStatus bool) (types.NodeStatus, error) //perm:read
	// StartTime returns node start time
	StartTime(context.Context) (time.Time, error) //perm:read
}
