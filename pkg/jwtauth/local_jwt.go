package jwtauth

import (
	"context"
	"crypto/rand"
	vjc "github.com/filecoin-project/venus-auth/cmd/jwtclient"
	"github.com/filecoin-project/venus/app/client/funcrule"
	"io"
	"io/ioutil"
	"strings"

	"github.com/filecoin-project/go-jsonrpc/auth"
	jwt3 "github.com/gbrlsnchs/jwt/v3"
	logging "github.com/ipfs/go-log"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/repo"
)

type APIAlg jwt3.HMACSHA

var jwtLog = logging.Logger("jwt")

type JwtPayload struct {
	Allow []auth.Permission
}

// JwtAuth auth through local token
type JwtAuth struct {
	apiSecret     *APIAlg
	jwtSecetName  string
	jwtHmacSecret string
	payload       JwtPayload
	lr            repo.Repo
}

func NewJwtAuth(lr repo.Repo) (vjc.IJwtAuthClient, error) {
	jwtAuth := &JwtAuth{
		jwtSecetName:  "auth-jwt-private",
		jwtHmacSecret: "jwt-hmac-secret",
		lr:            lr,
		payload:       JwtPayload{Allow: funcrule.AllPermissions},
	}
	var err error
	jwtAuth.apiSecret, err = jwtAuth.loadAPISecret()
	if err != nil {
		return nil, err
	}
	return vjc.IJwtAuthClient(jwtAuth), nil
}

func (jwtAuth *JwtAuth) loadAPISecret() (*APIAlg, error) {
	sk, err := jwtAuth.lr.Keystore().Get(jwtAuth.jwtHmacSecret)
	// todo use custome keystore to replace
	if err != nil && strings.Contains(err.Error(), "no key by the given name was found") {
		jwtLog.Warn("Generating new API secret")

		sk, err = ioutil.ReadAll(io.LimitReader(rand.Reader, 32))
		if err != nil {
			return nil, err
		}

		if err := jwtAuth.lr.Keystore().Put(jwtAuth.jwtHmacSecret, sk); err != nil {
			return nil, xerrors.Wrap(err, "failed to store private key")
		}

		cliToken, err := jwt3.Sign(&jwtAuth.payload, jwt3.NewHS256(sk))
		if err != nil {
			return nil, err
		}

		if err := jwtAuth.lr.SetAPIToken(cliToken); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, xerrors.Errorf("could not get JWT Token: %v", err)
	}

	return (*APIAlg)(jwt3.NewHS256(sk)), nil
}

// Verify verify token from request
func (jwtAuth *JwtAuth) Verify(ctx context.Context, token string) ([]auth.Permission, error) {
	var payload JwtPayload
	if _, err := jwt3.Verify([]byte(token), (*jwt3.HMACSHA)(jwtAuth.apiSecret), &payload); err != nil {
		return nil, xerrors.Errorf("JWT Verification failed: %v", err)
	}
	return payload.Allow, nil
}

type JwtAuthAPI struct { // nolint
	JwtAuth *JwtAuth
}

// API jwt for token api
// todo remove auth new api, because not secure for remote mod
func (jwtAuth *JwtAuth) API() vjc.IJwtAuthAPI {
	return &JwtAuthAPI{JwtAuth: jwtAuth}
}

func (jwtAuth *JwtAuth) V0API() vjc.IJwtAuthAPI {
	return &JwtAuthAPI{JwtAuth: jwtAuth}
}

// Verify check the token is valid or not
func (a *JwtAuthAPI) Verify(ctx context.Context, token string) ([]auth.Permission, error) {
	var payload JwtPayload
	if _, err := jwt3.Verify([]byte(token), (*jwt3.HMACSHA)(a.JwtAuth.apiSecret), &payload); err != nil {
		return nil, xerrors.Errorf("JWT Verification failed: %v", err)
	}
	return payload.Allow, nil
}

// AuthNew create new token with specify permission for access venus
func (a *JwtAuthAPI) AuthNew(ctx context.Context, perms []auth.Permission) ([]byte, error) {
	p := JwtPayload{
		Allow: perms, // TODO: consider checking validity
	}
	return jwt3.Sign(&p, (*jwt3.HMACSHA)(a.JwtAuth.apiSecret))
}
