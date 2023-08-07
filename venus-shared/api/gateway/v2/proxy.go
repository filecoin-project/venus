package gateway

import (
	"context"

	gatewayTypes "github.com/filecoin-project/venus/venus-shared/types/gateway"
)

type IProxy interface {
	RegisterReverse(ctx context.Context, hostKey gatewayTypes.HostKey, address string) error //perm:admin
}
