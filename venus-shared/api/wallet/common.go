package wallet

import (
	"context"

	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type ICommon interface {
	// Auth
	AuthVerify(ctx context.Context, token string) ([]auth.Permission, error) //perm:read
	AuthNew(ctx context.Context, perms []auth.Permission) ([]byte, error)    //perm:admin

	// Version provides information about API provider
	Version(context.Context) (types.Version, error) //perm:read

	LogList(context.Context) ([]string, error)         //perm:read
	LogSetLevel(context.Context, string, string) error //perm:write
}
