package client

import (
	"context"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"net/http"
)

func getVenusClientInfo(api, token string) (string, http.Header, error) {
	headers := http.Header{}
	headers.Add("Authorization", "Bearer "+token)
	apima, err := multiaddr.NewMultiaddr(api)
	if err != nil {
		return "", nil, err
	}

	_, addr, err := manet.DialArgs(apima)
	if err != nil {
		return "", nil, err
	}

	addr = "ws://" + addr + "/rpc/v0"
	return addr, headers, nil
}

//NewFullNode It is used to construct a full node access client.
//The API can be obtained from ~ /. Venus / API file, read from ~ /. Venus / token in local JWT mode,
//and obtained from Venus auth service in central authorization mode.
func NewFullNode(ctx context.Context, url, token string) (FullNodeStruct, jsonrpc.ClientCloser, error) {
	addr, headers, err := getVenusClientInfo(url, token)
	if err != nil {
		return FullNodeStruct{}, nil, err
	}
	node := FullNodeStruct{}
	closer, err := jsonrpc.NewClient(ctx, addr, "Filecoin", &node, headers)
	if err != nil {
		return FullNodeStruct{}, nil, err
	}
	return node, closer, nil
}
