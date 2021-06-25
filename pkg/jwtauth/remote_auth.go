package jwtauth

import (
	"context"

	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/filecoin-project/venus-auth/cmd/jwtclient"
	"github.com/filecoin-project/venus-auth/core"
	"github.com/filecoin-project/venus/app/submodule/apiface"
)

var _ IJwtAuthClient = (*RemoteAuth)(nil)

//RemoteAuth  in remote verification mode, venus connects venus-auth service, and verifies whether token is legal through rpc
type RemoteAuth struct {
	remoteClient *jwtclient.JWTClient
}

//NewRemoteAuth new remote auth client from venus-auth url
func NewRemoteAuth(url string) *RemoteAuth {
	return &RemoteAuth{
		remoteClient: jwtclient.NewJWTClient(url),
	}
}

//Verify check token through venus-auth rpc api
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

//API remote a new api
func (r *RemoteAuth) API() apiface.IJwtAuthAPI {
	return &remoteJwtAuthAPI{JwtAuth: r}
}

type remoteJwtAuthAPI struct { //nolint
	JwtAuth IJwtAuthClient
}

func (a *remoteJwtAuthAPI) Verify(ctx context.Context, spanID, serviceName, preHost, host, token string) ([]auth.Permission, error) {
	return a.JwtAuth.Verify(ctx, spanID, serviceName, preHost, host, token)
}

func (a *remoteJwtAuthAPI) AuthNew(ctx context.Context, perms []auth.Permission) ([]byte, error) {
	panic("not support new auth in remote auth mode")
}
