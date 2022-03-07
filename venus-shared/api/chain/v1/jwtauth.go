package v1

import (
	"context"

	"github.com/filecoin-project/go-jsonrpc/auth"
)

type IJwtAuthAPI interface {
	Verify(ctx context.Context, token string) ([]auth.Permission, error)  //perm:read
	AuthNew(ctx context.Context, perms []auth.Permission) ([]byte, error) //perm:admin
}
