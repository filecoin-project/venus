package client

import (
	"context"
	"github.com/filecoin-project/go-jsonrpc"
	v0 "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"

	"net/http"
)

func NewFullRPCV0(ctx context.Context, addr string, header http.Header) (v0.FullNode, jsonrpc.ClientCloser, error) {
	var full v0.FullNodeStruct
	closer, err := jsonrpc.NewClient(ctx, addr, "Filecoin", &full,
		header)
	return &full, closer, err
}

func NewFullRPCV1(ctx context.Context, addr string, header http.Header) (v1.FullNode, jsonrpc.ClientCloser, error) {
	var full v1.FullNodeStruct
	closer, err := jsonrpc.NewClient(ctx, addr, "Filecoin", &full,
		header)
	return &full, closer, err
}

func NewWalletRPC(ctx context.Context, addr string, header http.Header) (v1.IWallet, jsonrpc.ClientCloser, error) {
	var wallet v1.IWalletStruct
	closer, err := jsonrpc.NewClient(ctx, addr, "Filecoin", &wallet, header)
	return &wallet, closer, err
}
