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
	remote *vjc.JWTClient
}

// NewRemoteAuth new remote auth client from venus-auth url
func NewRemoteAuth(url string) *RemoteAuth {
	return &RemoteAuth{
		remote: vjc.NewJWTClient(url),
	}
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

func (r *RemoteAuth) GetUserLimit(name string) (*ratelimit.Limit, error) {
	res, err := r.remote.GetUser(&va.GetUserRequest{Name: name})
	if err != nil {
		return nil, err
	}
	return &ratelimit.Limit{
		Account: res.Name, Cap: res.ReqLimit.Cap, Duration: res.ReqLimit.ResetDur}, nil
}

func (r *RemoteAuth) ListUserLimits() ([]*ratelimit.Limit, error) {
	const PageSize = 5

	var limits = make([]*ratelimit.Limit, 0, PageSize*2)

	req := &va.ListUsersRequest{
		Page:       &core.Page{Skip: 0, Limit: PageSize},
		SourceType: 0, State: 0, KeySum: 0}

	for int64(len(limits)) == req.Skip {
		res, err := r.remote.ListUsers(req)
		if err != nil {
			return nil, err
		}
		for _, u := range res {
			limits = append(limits,
				&ratelimit.Limit{Account: u.Name, Cap: u.ReqLimit.Cap, Duration: u.ReqLimit.ResetDur})
		}

		req.Skip += PageSize
	}
	return limits, nil

}

// API remote a new api
func (r *RemoteAuth) API() vjc.IJwtAuthAPI {
	return &remoteJwtAuthAPI{JwtAuth: r}
}

func (r *RemoteAuth) V0API() vjc.IJwtAuthAPI {
	return &remoteJwtAuthAPI{JwtAuth: r}
}

type remoteJwtAuthAPI struct { // nolint
	JwtAuth vjc.IJwtAuthClient
}

func (a *remoteJwtAuthAPI) Verify(ctx context.Context, token string) ([]auth.Permission, error) {
	return a.JwtAuth.Verify(ctx, token)
}

func (a *remoteJwtAuthAPI) AuthNew(ctx context.Context, _ []auth.Permission) ([]byte, error) {
	panic("not support new auth in remote auth mode")
}
