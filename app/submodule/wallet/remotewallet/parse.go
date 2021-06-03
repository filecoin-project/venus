package remotewallet

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

var (
	regJWTToken = regexp.MustCompile("[a-zA-Z0-9\\-_]{5,}\\.[a-zA-Z0-9\\-_]{5,}\\.[a-zA-Z0-9\\-_]{5,}")                                                                                                                            //nolint
	regUUID     = regexp.MustCompile("[0-9a-fA-F]{8}\\-[0-9a-fA-F]{4}\\-[0-9a-fA-F]{4}\\-[0-9a-fA-F]{4}\\-[0-9a-fA-F]{12}")                                                                                                        //nolint
	regIPv4     = regexp.MustCompile("/ip4/(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)/tcp/[0-9]{4,5}/http") //nolint
)

const (
	ServiceToken        = "Authorization"
	WalletStrategyToken = "StrategyToken"
)

// APIInfo parse URL string to
type APIInfo struct {
	Addr          multiaddr.Multiaddr
	Token         []byte
	StrategyToken []byte
}

func ParseAPIInfo(s string) (*APIInfo, error) {
	token := []byte(regJWTToken.FindString(s))
	strategyToken := []byte(regUUID.FindString(s))
	addr := regIPv4.FindString(s)
	apima, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return nil, err
	}
	return &APIInfo{
		Addr:          apima,
		Token:         token,
		StrategyToken: strategyToken,
	}, nil
}

func (a APIInfo) DialArgs() (string, error) {
	_, addr, err := manet.DialArgs(a.Addr)
	if strings.HasPrefix(addr, "0.0.0.0:") {
		addr = "127.0.0.1:" + addr[8:]
	}
	return "ws://" + addr + "/rpc/v0", err
}

func (a APIInfo) AuthHeader() http.Header {
	if len(a.Token) != 0 {
		headers := http.Header{}
		headers.Add(ServiceToken, "Bearer "+string(a.Token))
		headers.Add(WalletStrategyToken, string(a.StrategyToken))
		return headers
	}
	return nil
}
