package jwtauth

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/filecoin-project/go-jsonrpc/auth"
	jwt3 "github.com/gbrlsnchs/jwt/v3"
	logging "github.com/ipfs/go-log"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/repo"
)

type APIAlg jwt3.HMACSHA

var jwtLog = logging.Logger("jwt")

var (
	ErrKeyInfoNotFound = fmt.Errorf("key info not found")
)

type JwtPayload struct {
	Allow []auth.Permission
}

type IJwtAuthAPI interface {
	Verify(ctx context.Context, spanID, serviceName, preHost, host, token string) ([]auth.Permission, error)
	AuthNew(ctx context.Context, perms []auth.Permission) ([]byte, error)
}

type IJwtAuthClient interface {
	API() IJwtAuthAPI
	Verify(ctx context.Context, spanID, serviceName, preHost, host, token string) ([]auth.Permission, error)
}

type JwtAuth struct {
	apiSecret     *APIAlg
	jwtSecetName  string
	jwtHmacSecret string
	payload       JwtPayload
	lr            repo.Repo
}

func NewJwtAuth(lr repo.Repo) (*JwtAuth, error) {
	jwtAuth := &JwtAuth{
		jwtSecetName:  "auth-jwt-private",
		jwtHmacSecret: "jwt-hmac-secret",
		lr:            lr,
		payload:       JwtPayload{Allow: []auth.Permission{"all"}},
	}
	var err error
	jwtAuth.apiSecret, err = jwtAuth.loadAPISecret()
	if err != nil {
		return nil, err
	}
	return jwtAuth, nil
}

func (jwtAuth *JwtAuth) loadAPISecret() (*APIAlg, error) {
	setAPIToken := func() error {
		secret, err := hex.DecodeString(jwtAuth.lr.Config().Jwt.Secret)
		cliToken, err := jwt3.Sign(&jwtAuth.payload, jwt3.NewHS256(secret))
		if err != nil {
			return err
		}

		if err := jwtAuth.lr.SetAPIToken(cliToken); err != nil {
			return err
		}
		return nil
	}

	if len(jwtAuth.lr.Config().Jwt.Secret) != 0 {
		secret, err := hex.DecodeString(jwtAuth.lr.Config().Jwt.Secret)
		if err != nil {
			return nil, err
		}
		if err := setAPIToken(); err != nil {
			return nil, err
		}
		return (*APIAlg)(jwt3.NewHS256(secret)), nil
	}

	pk, err := jwtAuth.lr.Keystore().Get(jwtAuth.jwtHmacSecret)
	//todo use custome keystore to replace
	if err != nil && strings.Contains(err.Error(), "no key by the given name was found") {
		if err := setAPIToken(); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, xerrors.Errorf("could not get JWT Token: %v", err)
	}
	raw, _ := pk.Raw()
	return (*APIAlg)(jwt3.NewHS256(raw)), nil
}

func (jwtAuth *JwtAuth) Verify(ctx context.Context, spanID, serviceName, preHost, host, token string) ([]auth.Permission, error) {
	var payload JwtPayload
	if _, err := jwt3.Verify([]byte(token), (*jwt3.HMACSHA)(jwtAuth.apiSecret), &payload); err != nil {
		return nil, xerrors.Errorf("JWT Verification failed: %v", err)
	}

	return payload.Allow, nil
}

type JwtAuthAPI struct { //nolint
	JwtAuth *JwtAuth
}

func (jwtAuth *JwtAuth) API() IJwtAuthAPI {
	return &JwtAuthAPI{JwtAuth: jwtAuth}
}

func (a *JwtAuthAPI) Verify(ctx context.Context, spanID, serviceName, preHost, host, token string) ([]auth.Permission, error) {
	var payload JwtPayload
	if _, err := jwt3.Verify([]byte(token), (*jwt3.HMACSHA)(a.JwtAuth.apiSecret), &payload); err != nil {
		return nil, xerrors.Errorf("JWT Verification failed: %v", err)
	}

	return payload.Allow, nil
}

func (a *JwtAuthAPI) AuthNew(ctx context.Context, perms []auth.Permission) ([]byte, error) {
	p := JwtPayload{
		Allow: perms, // TODO: consider checking validity
	}

	return jwt3.Sign(&p, (*jwt3.HMACSHA)(a.JwtAuth.apiSecret))
}
