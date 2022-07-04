package jwtauth

import (
	"context"

	"github.com/filecoin-project/go-jsonrpc/auth"
	va "github.com/filecoin-project/venus-auth/auth"
	vjc "github.com/filecoin-project/venus-auth/cmd/jwtclient"
	"github.com/filecoin-project/venus-auth/core"
	"github.com/ipfs-force-community/metrics/ratelimit"
)

var _ vjc.IJwtAuthClient = (*RemoteAuth)(nil)
var _ ratelimit.ILimitFinder = (*RemoteAuth)(nil)

type ValueFromCtx struct{}

func (vfc *ValueFromCtx) AccFromCtx(ctx context.Context) (string, bool) {
	return vjc.CtxGetName(ctx)
}
func (vfc *ValueFromCtx) HostFromCtx(ctx context.Context) (string, bool) {
	return vjc.CtxGetTokenLocation(ctx)
}

// RemoteAuth  in remote verification mode, venus connects venus-auth service, and verifies whether token is legal through rpc
type RemoteAuth struct {
	remote *vjc.AuthClient
}

// NewRemoteAuth new remote auth client from venus-auth url
func NewRemoteAuth(url string) *RemoteAuth {
	// vjc.NewAuthClient always return nil error
	cli, _ := vjc.NewAuthClient(url)
	return &RemoteAuth{remote: cli}
}

// Verify check token through venus-auth rpc api
func (r *RemoteAuth) Verify(ctx context.Context, token string) ([]auth.Permission, error) {
	var perms []auth.Permission
	var err error

	var res *va.VerifyResponse
	if res, err = r.remote.Verify(ctx, token); err != nil {
		return nil, err
	}

	jwtPerms := core.AdaptOldStrategy(res.Perm)
	perms = make([]auth.Permission, len(jwtPerms))
	copy(perms, jwtPerms)

	return perms, nil
}

func (r *RemoteAuth) GetUserLimit(name, service, api string) (*ratelimit.Limit, error) {
	res, err := r.remote.GetUserRateLimit(name, "")
	if err != nil {
		return nil, err
	}

	var limit = &ratelimit.Limit{Account: name, Cap: 0, Duration: 0}
	if l := res.MatchedLimit(service, api); l != nil {
		limit.Cap = l.ReqLimit.Cap
		limit.Duration = l.ReqLimit.ResetDur
	}

	return limit, nil
}
