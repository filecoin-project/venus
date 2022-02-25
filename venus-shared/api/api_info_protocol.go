package api

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/multiformats/go-multiaddr"
)

const ProtoVersion = multiaddr.P_WSS + 1

func init() {
	err := multiaddr.AddProtocol(multiaddr.Protocol{
		Name:  "version",
		Code:  ProtoVersion,
		VCode: multiaddr.CodeToVarint(ProtoVersion),
		Size:  multiaddr.LengthPrefixedVarSize,
		Transcoder: multiaddr.NewTranscoderFromFunctions(func(s string) ([]byte, error) {
			if !strings.HasPrefix(s, "v") {
				return nil, fmt.Errorf("version must start with version prefix v")
			}
			if len(s) < 2 {
				return nil, fmt.Errorf("must give a specify version such as v0")
			}
			_, err := strconv.Atoi(s[1:])
			if err != nil {
				return nil, fmt.Errorf("version part must be number")
			}
			return []byte(s), nil
		}, func(bytes []byte) (string, error) {
			vStr := string(bytes)
			if !strings.HasPrefix(vStr, "v") {
				return "", fmt.Errorf("version must start with version prefix v")
			}
			if len(vStr) < 2 {
				return "", fmt.Errorf("must give a specify version such as v0")
			}
			_, err := strconv.Atoi(vStr[1:])
			if err != nil {
				return "", fmt.Errorf("version part must be number")
			}
			return vStr, nil
		}, nil),
	})

	if err != nil {
		panic(fmt.Errorf("add `version` protocol into multiaddr: %w", err))
	}
}
