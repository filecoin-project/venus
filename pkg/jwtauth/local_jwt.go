package jwtauth

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	vjc "github.com/filecoin-project/venus-auth/cmd/jwtclient"
	"github.com/filecoin-project/venus/venus-shared/api/permission"

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
		payload:       JwtPayload{Allow: permission.AllPermissions},
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
	// todo use customer keystore to replace
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
		return nil, fmt.Errorf("could not get JWT Token: %v", err)
	}

	return (*APIAlg)(jwt3.NewHS256(sk)), nil
}

// Verify verify token from request
func (jwtAuth *JwtAuth) Verify(ctx context.Context, token string) ([]auth.Permission, error) {
	var payload JwtPayload
	if _, err := jwt3.Verify([]byte(token), (*jwt3.HMACSHA)(jwtAuth.apiSecret), &payload); err != nil {
		return nil, fmt.Errorf("JWT Verification failed: %v", err)
	}
	return payload.Allow, nil
}
