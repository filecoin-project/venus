package v0

import (
	"context"
	"time"

	"github.com/filecoin-project/venus/venus-shared/api"
)

type ICommon interface {
	api.Version
	// StartTime returns node start time
	StartTime(context.Context) (time.Time, error) //perm:read
}
