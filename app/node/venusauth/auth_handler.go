package venusauth

import (
	"github.com/ipfs-force-community/venus-auth/auth"
	"github.com/ipfs-force-community/venus-auth/core"
	"github.com/ipfs-force-community/venus-auth/util"
	logging "github.com/ipfs/go-log/v2"
	"net/http"
	"strings"
)

var log = logging.Logger("venusauth")

type Handler struct {
	Verify func(spanId, serviceName, preHost, host, token string) (*auth.VerifyResponse, error)
	Next   http.HandlerFunc
}

func (h *Handler) Auth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.Header.Get("Authorization")
	// if other nodes on the same PC, the permission check will passes directly
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
				w.WriteHeader(401)
				return
			}
			token = strings.TrimPrefix(token, "Bearer ")
			res, err := h.Verify(util.MacAddr(), "venus", r.RemoteAddr, r.Host, token)
			if err != nil {
				log.Warnf("JWT Verification failed (originating from %s): %s", r.RemoteAddr, err)
				w.WriteHeader(401)
				return
			}
			ctx = core.WithPerm(ctx, res.Perm)
		}
	}
	r = r.WithContext(ctx)
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Auth(w, r)
	h.Next(w, r)
}
