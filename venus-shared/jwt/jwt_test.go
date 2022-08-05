package jwt

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/filecoin-project/venus-auth/auth"
	venusauth "github.com/filecoin-project/venus-auth/auth"
	"github.com/filecoin-project/venus-auth/config"
	"github.com/filecoin-project/venus-auth/core"
	"github.com/stretchr/testify/assert"
)

func TestLocalAuthClient(t *testing.T) {
	secret, err := RandSecret()
	assert.NoError(t, err)

	payload := venusauth.JWTPayload{
		Perm: core.PermAdmin,
		Name: "MarketLocalToken",
	}

	client, token, err := NewLocalAuthClient(secret, payload)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(token)
	ctx := context.Background()
	permissions, err := client.Verify(ctx, string(token))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(permissions)
}

func TestRmoteAuthClient(t *testing.T) {

	cnf, err := config.DefaultConfig()
	if err != nil {
		log.Fatalf("failed to get default config err:%s", err)
	}

	var tmpPath string

	if tmpPath, err = ioutil.TempDir("", "auth-serve"); err != nil {
		log.Fatalf("failed to create temp dir err:%s", err)
	}
	defer os.RemoveAll(tmpPath)

	app, err := venusauth.NewOAuthApp("", tmpPath, cnf.DB)
	if err != nil {
		log.Fatalf("Failed to init oauthApp : %s", err)
	}
	router := auth.InitRouter(app)
	server := &http.Server{
		Addr:         ":" + cnf.Port,
		Handler:      router,
		ReadTimeout:  cnf.ReadTimeout,
		WriteTimeout: cnf.WriteTimeout,
		IdleTimeout:  cnf.IdleTimeout,
	}

	venusauth.o

	go func() {
		log.Infof("server start and listen on %s", cnf.Port)
		_ = server.ListenAndServe()
	}() //nolint

	cli, err := NewRemoteClient("http://localhost:" + cnf.Port)
	if err != nil {
		log.Fatalf("create auth client failed:%s\n", err.Error())
		return
	}

	defer func() {
		log.Info("close auth service")
		_ = server.Close()
	}()

}
