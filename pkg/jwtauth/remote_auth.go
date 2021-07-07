package jwtauth

import (
	"context"
	"github.com/filecoin-project/go-jsonrpc/auth"
	auth2 "github.com/filecoin-project/venus-auth/auth"
	vjc "github.com/filecoin-project/venus-auth/cmd/jwtclient"
	"github.com/filecoin-project/venus-auth/core"
)

var _ vjc.IJwtAuthClient = (*RemoteAuth)(nil)

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

	var res *auth2.VerifyResponse
	if res, err = r.remote.Verify(ctx, token); err != nil {
		return nil, err
	}

	jwtPerms := core.AdaptOldStrategy(res.Perm)
	perms = make([]auth.Permission, len(jwtPerms))
	copy(perms, jwtPerms)

	return perms, nil
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
