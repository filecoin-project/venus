package jwtauth

import (
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/filecoin-project/venus-auth/util"
	ipfsHttp "github.com/ipfs/go-ipfs-cmds/http"
	logging "github.com/ipfs/go-log"
	"net/http"
	"strings"
)

var log = logging.Logger("venusauth")

type AuthMux struct {
	mux    *http.ServeMux
	jwtCli IJwtAuthClient

	trustHandle map[string]http.Handler
}

func NewAuthMux(jwtCli IJwtAuthClient, serveMux *http.ServeMux) *AuthMux {
	return &AuthMux{mux: serveMux, jwtCli: jwtCli, trustHandle: make(map[string]http.Handler)}
}

func (authMux *AuthMux) TrustHandle(pattern string, handler http.Handler) {
	authMux.trustHandle[pattern] = handler
}

func (authMux *AuthMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handle, ok := authMux.trustHandle[r.RequestURI]; ok {
		handle.ServeHTTP(w, r)
		return
	}

	ctx := r.Context()
	token := r.Header.Get("Authorization")
	// if other nodes on the same PC, the permission check will passes directly
	//if strings.Split(r.RemoteAddr, ":")[0] == "127.0.0.1" {
	//	ctx = core.WithPerm(ctx, core.PermAdmin)
	//} else {
	if token == "" {
		token = r.FormValue("token")
		if token != "" {
			token = "Bearer " + token
		}
	}

	if !strings.HasPrefix(token, "Bearer ") {
		log.Warn("missing Bearer prefix in venusauth header")
		w.WriteHeader(401)
		return
	}

	token = strings.TrimPrefix(token, "Bearer ")
	res, err := authMux.jwtCli.Verify(r.Context(), util.MacAddr(), "venus", r.RemoteAddr, r.Host, token)
	if err != nil {
		log.Warnf("JWT Verification failed (originating from %s): %s", r.RemoteAddr, err)
		w.WriteHeader(401)
		return
	}
	ctx = auth.WithPerm(ctx, res)
	ctx = ipfsHttp.WithPerm(ctx, res)
	//}
	*r = *(r.WithContext(ctx))
	authMux.mux.ServeHTTP(w, r)
}
