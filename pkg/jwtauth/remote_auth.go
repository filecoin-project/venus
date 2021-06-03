package jwtauth

import (
	"context"

	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/filecoin-project/venus-auth/cmd/jwtclient"
	"github.com/filecoin-project/venus-auth/core"
	"github.com/filecoin-project/venus/app/submodule/apiface"
)

var _ IJwtAuthClient = (*RemoteAuth)(nil)

type RemoteAuth struct {
	remoteClient *jwtclient.JWTClient
}

func NewRemoteAuth(url string) *RemoteAuth {
	return &RemoteAuth{
		remoteClient: jwtclient.NewJWTClient(url),
	}
}

func (r *RemoteAuth) Verify(ctx context.Context, spanID, serviceName, preHost, host, token string) ([]auth.Permission, error) {
	res, err := r.remoteClient.Verify(spanID, serviceName, preHost, host, token)
	if err != nil {
		return nil, err
	}
	jwtPerms := core.AdaptOldStrategy(res.Perm)
	perms := make([]auth.Permission, len(jwtPerms))
	copy(perms, jwtPerms)
	return perms, nil
}

func (r *RemoteAuth) API() apiface.IJwtAuthAPI {
	return &RemoteJwtAuthAPI{JwtAuth: r}
}

func (r *RemoteAuth) V0API() apiface.IJwtAuthAPI {
	return &RemoteJwtAuthAPI{JwtAuth: r}
}

type RemoteJwtAuthAPI struct { //nolint
	JwtAuth IJwtAuthClient
}

func (a *RemoteJwtAuthAPI) Verify(ctx context.Context, spanID, serviceName, preHost, host, token string) ([]auth.Permission, error) {
	return a.JwtAuth.Verify(ctx, spanID, serviceName, preHost, host, token)
}

func (a *RemoteJwtAuthAPI) AuthNew(ctx context.Context, perms []auth.Permission) ([]byte, error) {
	panic("not support new auth in remote auth mode")
}
