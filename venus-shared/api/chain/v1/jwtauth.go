package v1

import (
	"context"
	"github.com/filecoin-project/go-jsonrpc/auth"
)

type IJwtAuthAPI interface {
	// Rule[perm:read]
	Verify(ctx context.Context, host, token string) ([]auth.Permission, error)
	// Rule[perm:read]
	AuthNew(ctx context.Context, perms []auth.Permission) ([]byte, error)
}
