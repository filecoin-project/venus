package api

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	multiaddr "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

var (
	infoWithToken = regexp.MustCompile("^[a-zA-Z0-9\\-_]+?\\.[a-zA-Z0-9\\-_]+?\\.([a-zA-Z0-9\\-_]+)?:.+$") // nolint:gosimple
)

func VerString(ver uint32) string {
	return fmt.Sprintf("v%d", ver)
}

type APIInfo struct { // nolint
	Addr  string
	Token []byte
}

func NewAPIInfo(addr, token string) APIInfo {
	return APIInfo{
		Addr:  addr,
		Token: []byte(token),
	}
}

func ParseApiInfo(s string) APIInfo {
	var tok []byte
	if infoWithToken.Match([]byte(s)) {
		sp := strings.SplitN(s, ":", 2)
		tok = []byte(sp[0])
		s = sp[1]
	}

	return APIInfo{
		Addr:  s,
		Token: tok,
	}
}

//DialArgs parser libp2p address to http/ws protocol, the version argument can be override by address in version
func (a APIInfo) DialArgs(version string) (string, error) {
	return DialArgs(a.Addr, version)
}

func (a APIInfo) Host() (string, error) {
	ma, err := multiaddr.NewMultiaddr(a.Addr)
	if err == nil {
		_, addr, err := manet.DialArgs(ma)
		if err != nil {
			return "", err
		}

		return addr, nil
	}

	spec, err := url.Parse(a.Addr)
	if err != nil {
		return "", err
	}
	return spec.Host, nil
}

func (a APIInfo) AuthHeader() http.Header {
	if len(a.Token) != 0 {
		headers := http.Header{}
		a.SetAuthHeader(headers)
		return headers
	}

	return nil
}

func (a APIInfo) SetAuthHeader(h http.Header) {
	if len(a.Token) != 0 {
		h.Add(AuthorizationHeader, "Bearer "+string(a.Token))
	}
}

func DialArgs(addr, version string) (string, error) {
	ma, err := multiaddr.NewMultiaddr(addr)
	if err == nil {
		_, addr, err := manet.DialArgs(ma)
		if err != nil {
			return "", fmt.Errorf("parser libp2p url fail %w", err)
		}

		//override version
		val, err := ma.ValueForProtocol(ProtoVersion)
		if err == nil {
			version = val
		} else if err != multiaddr.ErrProtocolNotFound {
			return "", err
		}

		_, err = ma.ValueForProtocol(multiaddr.P_WSS)
		if err == nil {
			return "wss://" + addr + "/rpc/" + version, nil
		} else if err != multiaddr.ErrProtocolNotFound {
			return "", err
		}

		_, err = ma.ValueForProtocol(multiaddr.P_HTTPS)
		if err == nil {
			return "https://" + addr + "/rpc/" + version, nil
		} else if err != multiaddr.ErrProtocolNotFound {
			return "", err
		}

		_, err = ma.ValueForProtocol(multiaddr.P_WS)
		if err == nil {
			return "ws://" + addr + "/rpc/" + version, nil
		} else if err != multiaddr.ErrProtocolNotFound {
			return "", err
		}

		_, err = ma.ValueForProtocol(multiaddr.P_HTTP)
		if err == nil {
			return "http://" + addr + "/rpc/" + version, nil
		} else if err != multiaddr.ErrProtocolNotFound {
			return "", err
		}

		return "ws://" + addr + "/rpc/" + version, nil
	}

	_, err = url.Parse(addr)
	if err != nil {
		return "", fmt.Errorf("parser address fail %w", err)
	}

	return strings.TrimRight(addr, "/") + "/rpc/" + version, nil
}
