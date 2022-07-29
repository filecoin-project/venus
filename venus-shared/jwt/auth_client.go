package jwt

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/filecoin-project/go-jsonrpc/auth"
	venusauth "github.com/filecoin-project/venus-auth/auth"
	"github.com/filecoin-project/venus-auth/core"
	jwt3 "github.com/gbrlsnchs/jwt/v3"
	"github.com/go-resty/resty/v2"
)

type IAuthClient interface {
	Verify(ctx context.Context, token string) ([]auth.Permission, error)
}

type RemoteAuthClient struct {
	*resty.Client
}

func NewRemoteClient(url string) (*RemoteAuthClient, error) {
	client := resty.New().
		SetHostURL(url).
		SetHeader("Accept", "application/json")
	return &RemoteAuthClient{Client: client}, nil
}

func (lc *RemoteAuthClient) Verify(ctx context.Context, token string) ([]auth.Permission, error) {
	resp, err := lc.R().SetContext(ctx).
		SetBody(venusauth.VerifyRequest{Token: token}).
		SetResult(&venusauth.VerifyResponse{}).Post("/verify")

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() == http.StatusOK {
		res := resp.Result().(*venusauth.VerifyResponse)
		return AdaptOldStrategy(res.Perm), nil
	}

	return nil, fmt.Errorf("response code is : %d, msg:%s", resp.StatusCode(), resp.Body())
}

type LocalAuthClient struct {
	alg *jwt3.HMACSHA
}

func NewLocalAuthClient(secret []byte, payload venusauth.JWTPayload) (*LocalAuthClient, []byte, error) {
	client := &LocalAuthClient{
		alg: jwt3.NewHS256(secret),
	}

	token, err := jwt3.Sign(payload, client.alg)
	return client, token, err
}

func (c *LocalAuthClient) Verify(ctx context.Context, token string) ([]auth.Permission, error) {
	var payload venusauth.JWTPayload
	_, err := jwt3.Verify([]byte(token), c.alg, &payload)
	if err != nil {
		return nil, err
	}

	return AdaptOldStrategy(payload.Perm), nil
}

func AdaptOldStrategy(perm string) []auth.Permission {
	jwtPerms := core.AdaptOldStrategy(perm)
	perms := make([]auth.Permission, len(jwtPerms))
	copy(perms, jwtPerms)
	return perms
}

func RandSecret() ([]byte, error) {
	return ioutil.ReadAll(io.LimitReader(rand.Reader, 32))
}
