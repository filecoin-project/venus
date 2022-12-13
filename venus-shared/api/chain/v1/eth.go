package v1

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type IETH interface {
	// These methods are used for Ethereum-compatible JSON-RPC calls
	//
	// EthAccounts will always return [] since we don't expect Lotus to manage private keys
	EthAccounts(ctx context.Context) ([]types.EthAddress, error) //perm:read
	// EthBlockNumber returns the height of the latest (heaviest) TipSet
	EthBlockNumber(ctx context.Context) (types.EthInt, error) //perm:read
	// EthGetBlockTransactionCountByNumber returns the number of messages in the TipSet
	EthGetBlockTransactionCountByNumber(ctx context.Context, blkNum types.EthInt) (types.EthInt, error) //perm:read
	// EthGetBlockTransactionCountByHash returns the number of messages in the TipSet
	EthGetBlockTransactionCountByHash(ctx context.Context, blkHash types.EthHash) (types.EthInt, error) //perm:read

	EthGetBlockByHash(ctx context.Context, blkHash types.EthHash, fullTxInfo bool) (types.EthBlock, error)                      //perm:read
	EthGetBlockByNumber(ctx context.Context, blkNum types.EthInt, fullTxInfo bool) (types.EthBlock, error)                      //perm:read
	EthGetTransactionByHash(ctx context.Context, txHash types.EthHash) (types.EthTx, error)                                     //perm:read
	EthGetTransactionCount(ctx context.Context, sender types.EthAddress, blkOpt string) (types.EthInt, error)                   //perm:read
	EthGetTransactionReceipt(ctx context.Context, blkHash types.EthHash) (types.EthTxReceipt, error)                            //perm:read
	EthGetTransactionByBlockHashAndIndex(ctx context.Context, blkHash types.EthHash, txIndex types.EthInt) (types.EthTx, error) //perm:read
	EthGetTransactionByBlockNumberAndIndex(ctx context.Context, blkNum types.EthInt, txIndex types.EthInt) (types.EthTx, error) //perm:read

	EthGetCode(ctx context.Context, address types.EthAddress) (string, error)                                              //perm:read
	EthGetStorageAt(ctx context.Context, address types.EthAddress, position types.EthInt, blkParam string) (string, error) //perm:read
	EthGetBalance(ctx context.Context, address types.EthAddress, blkParam string) (types.EthBigInt, error)                 //perm:read
	EthChainId(ctx context.Context) (types.EthInt, error)                                                                  //perm:read
	NetVersion(ctx context.Context) (string, error)                                                                        //perm:read
	NetListening(ctx context.Context) (bool, error)                                                                        //perm:read
	EthProtocolVersion(ctx context.Context) (types.EthInt, error)                                                          //perm:read
	EthGasPrice(ctx context.Context) (types.EthInt, error)                                                                 //perm:read
	EthMaxPriorityFeePerGas(ctx context.Context) (types.EthInt, error)                                                     //perm:read
	EthEstimateGas(ctx context.Context, tx types.EthCall, blkParam string) (types.EthInt, error)                           //perm:read
	EthCall(ctx context.Context, tx types.EthCall, blkParam string) (string, error)                                        //perm:read
}
