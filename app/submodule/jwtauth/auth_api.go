package jwtauth

import (
	"context"

	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/filecoin-project/venus/pkg/jwtauth"
	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

type JwtAuthAPI struct { // nolint
	*jwtauth.JwtAuth
}

func NewJwtAuthAPI(a *jwtauth.JwtAuth) *JwtAuthAPI {
	return &JwtAuthAPI{JwtAuth: a}
}

// Verify check the token is valid or not
func (a *JwtAuthAPI) Verify(ctx context.Context, token string) ([]auth.Permission, error) {
	return a.JwtAuth.Verify(ctx, token)
}

// AuthNew create new token with specify permission for access venus
func (a *JwtAuthAPI) AuthNew(ctx context.Context, perms []auth.Permission) ([]byte, error) {
	return a.JwtAuth.AuthNew(ctx, perms)
}

func (a *JwtAuthAPI) API() v1api.IJwtAuthAPI {
	return a
}

func (a *JwtAuthAPI) V0API() v0api.IJwtAuthAPI {
	return a
}
