package venusauth

import (
	"github.com/ipfs-force-community/venus-auth/auth"
	"github.com/ipfs-force-community/venus-auth/core"
	"github.com/ipfs-force-community/venus-auth/util"
	"net/http"
	"strings"
)

type HandlerWrapper struct {
	Verify func(spanId, serviceName, preHost, host, token string) (*auth.VerifyResponse, error)
}

func (h *HandlerWrapper) Wrapper(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r) // call original
		h.auth(w, r)
	})
}

func (h *HandlerWrapper) auth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.Header.Get("Authorization")
	if r.RemoteAddr[:len("127.0.0.1")] == "127.0.0.1" {
		ctx = core.WithPerm(ctx, core.PermAdmin)
	} else {
		if token == "" {
			token = r.FormValue("token")
			if token != "" {
				token = "Bearer " + token
			}
		}
		if token != "" {
			if !strings.HasPrefix(token, "Bearer ") {
				log.Warn("missing Bearer prefix in venusauth header")
				http.Error(w, "401 - Unauthorized", http.StatusUnauthorized)
				return
			}
			token = strings.TrimPrefix(token, "Bearer ")
			res, err := h.Verify(util.MacAddr(), "venus", r.RemoteAddr, r.Host, token)
			if err != nil {
				log.Warnf("JWT Verification failed (originating from %s): %s", r.RemoteAddr, err)
				http.Error(w, "401 - Unauthorized", http.StatusUnauthorized)
				return
			}
			ctx = core.WithPerm(ctx, res.Perm)
		}
	}
	r = r.WithContext(ctx)
}
