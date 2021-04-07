package node

import (
	orginalAuth "github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/filecoin-project/venus/app/node/venusauth"
	"github.com/ipfs-force-community/venus-auth/cmd/jwtclient"
	"net/http"
)

type CustomServe struct {
	*http.ServeMux
	rpcHandler     http.Handler
	restfulHandler http.Handler
}

func NewCustomServe(node *Node) *CustomServe {
	cs := &CustomServe{
		ServeMux: http.NewServeMux(),
	}
	venusAuthURL := node.repo.Config().API.VenusAuthURL
	log.Info("venus auth url ", venusAuthURL)
	var ah http.Handler
	if venusAuthURL == "" {
		jwtAuth := node.jwtAuth.API()
		ah = &orginalAuth.Handler{
			Verify: jwtAuth.AuthVerify,
			Next:   node.jsonRPCService.ServeHTTP,
		}
	} else {
		jwtCli := jwtclient.NewJWTClient(venusAuthURL)
		ah = &venusauth.Handler{
			Verify: jwtCli.Verify,
			Next:   node.jsonRPCService.ServeHTTP,
		}
	}
	cs.rpcHandler = cs.Wrapper(ah)
	cs.restfulHandler = cs.Wrapper2(ah.ServeHTTP)
	return cs
}

func (cs *CustomServe) Wrapper(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	})
}

func (cs *CustomServe) Wrapper2(hf http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hf.ServeHTTP(w, r)
	})
}
