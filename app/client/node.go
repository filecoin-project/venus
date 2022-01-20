package client

import (
	"context"
	"io/ioutil"
	"net/http"
	"path"

	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/ipfs-force-community/venus-common-utils/apiinfo"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus/app/paths"
)

func getVenusClientInfo(version string) (string, http.Header, error) {
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

	apiInfo := apiinfo.NewAPIInfo(string(rpcBytes), string(tokenBytes))
	addr, err := apiInfo.DialArgs("v0")
	if err != nil {
		return "", nil, err
	}
	return addr, apiInfo.AuthHeader(), nil
}

//NewFullNode It is used to construct a full node access client.
//The API can be obtained from ~ /. Venus / API file, read from ~ /. Venus / token in local JWT mode,
//and obtained from Venus auth service in central authorization mode.
func GetFullNodeAPI(ctx context.Context, version string) (v1api.FullNodeStruct, jsonrpc.ClientCloser, error) {
	addr, headers, err := getVenusClientInfo(version)
	if err != nil {
		return v1api.FullNodeStruct{}, nil, err
	}

	node := v1api.FullNodeStruct{}
	closer, err := jsonrpc.NewClient(ctx, addr, "Filecoin", &node, headers)
	if err != nil {
		return v1api.FullNodeStruct{}, nil, err
	}

	return node, closer, nil
}
