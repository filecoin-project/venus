package client

import (
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus/app/paths"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"io/ioutil"
	"net/http"
	"path"
)

func getVenusClientInfo() (string, http.Header, error) {
	repoPath, err := paths.GetRepoPath("")
	if err != nil {
		return "", nil, err
	}

	tokePath := path.Join(repoPath, "token")
	rpcPath := path.Join(repoPath, "api")

	tokenBytes, err := ioutil.ReadFile(tokePath)
	if err != nil {
		return "", nil, err
	}
	rpcBytes, err := ioutil.ReadFile(rpcPath)
	if err != nil {
		return "", nil, err
	}

	headers := http.Header{}
	headers.Add("Authorization", "Bearer "+string(tokenBytes))
	apima, err := multiaddr.NewMultiaddr(string(rpcBytes))
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

func NewFullNode() (FullNode, jsonrpc.ClientCloser, error) {
	addr, headers, err := getVenusClientInfo()
	if err != nil {
		return FullNode{}, nil, err
	}
	node := FullNode{}
	closer, err := jsonrpc.NewClient(addr, "Filecoin", &node, headers)
	if err != nil {
		return FullNode{}, nil, err
	}
	return node, closer, nil
}

func NewMiningAPINode() (MiningAPI, jsonrpc.ClientCloser, error) {
	addr, headers, err := getVenusClientInfo()
	if err != nil {
		return MiningAPI{}, nil, err
	}
	node := MiningAPI{}
	closer, err := jsonrpc.NewClient(addr, "Filecoin", &node, headers)
	if err != nil {
		return MiningAPI{}, nil, err
	}
	return node, closer, nil
}
