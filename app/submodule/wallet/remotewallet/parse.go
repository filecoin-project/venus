package remotewallet

import (
	"net/http"
	"regexp"

	"github.com/ipfs-force-community/venus-common-utils/apiinfo"
)

var (
	regJWTToken = regexp.MustCompile(`[a-zA-Z0-9\-_]{5,}\.[a-zA-Z0-9\-_]{5,}\.[a-zA-Z0-9\-_]{5,}`)
	regUUID     = regexp.MustCompile(`[0-9a-fA-F]{8}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{12}`)
	regIPv4     = regexp.MustCompile(`/ip4/(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)/tcp/[0-9]{4,5}/http`)
)

const (
	ServiceToken        = "Authorization"
	WalletStrategyToken = "StrategyToken"
)

// APIInfo parse URL string to
type APIInfo struct {
	Addr          string
	Token         []byte
	StrategyToken []byte
}

func ParseAPIInfo(s string) (*APIInfo, error) {
	token := []byte(regJWTToken.FindString(s))
	strategyToken := []byte(regUUID.FindString(s))
	addr := regIPv4.FindString(s)
	return &APIInfo{
		Addr:          addr,
		Token:         token,
		StrategyToken: strategyToken,
	}, nil
}

func (a APIInfo) DialArgs() (string, error) {
	return apiinfo.DialArgs(a.Addr, "v0")
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
