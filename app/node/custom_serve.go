package node

import (
	orginalAuth "github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/filecoin-project/venus/app/node/venusauth"
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
	var ah http.Handler
	if node.jwtCli == nil {
		log.Info("local jwt handler")
		jwtAuth := node.jwtAuth.API()
		ah = &orginalAuth.Handler{
			Verify: jwtAuth.AuthVerify,
			Next:   node.jsonRPCService.ServeHTTP,
		}
		cs.restfulHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		})

	} else {
		log.Info("venus auth handler")
		vah := &venusauth.Handler{
			Verify: node.jwtCli.Verify,
			Next:   node.jsonRPCService.ServeHTTP,
		}
		cs.restfulHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			vah.Auth(w, r)
		})
		ah = vah
	}
	cs.rpcHandler = cs.Wrapper(ah)

	return cs
}

func (cs *CustomServe) Wrapper(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	})
}
