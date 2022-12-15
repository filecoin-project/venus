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
	EthBlockNumber(ctx context.Context) (types.EthUint64, error) //perm:read
	// EthGetBlockTransactionCountByNumber returns the number of messages in the TipSet
	EthGetBlockTransactionCountByNumber(ctx context.Context, blkNum types.EthUint64) (types.EthUint64, error) //perm:read
	// EthGetBlockTransactionCountByHash returns the number of messages in the TipSet
	EthGetBlockTransactionCountByHash(ctx context.Context, blkHash types.EthHash) (types.EthUint64, error) //perm:read

	EthGetBlockByHash(ctx context.Context, blkHash types.EthHash, fullTxInfo bool) (types.EthBlock, error)                            //perm:read
	EthGetBlockByNumber(ctx context.Context, blkNum string, fullTxInfo bool) (types.EthBlock, error)                                  //perm:read
	EthGetTransactionByHash(ctx context.Context, txHash *types.EthHash) (*types.EthTx, error)                                         //perm:read
	EthGetTransactionCount(ctx context.Context, sender types.EthAddress, blkOpt string) (types.EthUint64, error)                      //perm:read
	EthGetTransactionReceipt(ctx context.Context, txHash types.EthHash) (*types.EthTxReceipt, error)                                  //perm:read
	EthGetTransactionByBlockHashAndIndex(ctx context.Context, blkHash types.EthHash, txIndex types.EthUint64) (types.EthTx, error)    //perm:read
	EthGetTransactionByBlockNumberAndIndex(ctx context.Context, blkNum types.EthUint64, txIndex types.EthUint64) (types.EthTx, error) //perm:read

	EthGetCode(ctx context.Context, address types.EthAddress) (types.EthBytes, error)                                                        //perm:read
	EthGetStorageAt(ctx context.Context, address types.EthAddress, position types.EthBytes, blkParam string) (types.EthBytes, error)         //perm:read
	EthGetBalance(ctx context.Context, address types.EthAddress, blkParam string) (types.EthBigInt, error)                                   //perm:read
	EthChainId(ctx context.Context) (types.EthUint64, error)                                                                                 //perm:read
	NetVersion(ctx context.Context) (string, error)                                                                                          //perm:read
	NetListening(ctx context.Context) (bool, error)                                                                                          //perm:read
	EthProtocolVersion(ctx context.Context) (types.EthUint64, error)                                                                         //perm:read
	EthGasPrice(ctx context.Context) (types.EthBigInt, error)                                                                                //perm:read
	EthFeeHistory(ctx context.Context, blkCount types.EthUint64, newestBlk string, rewardPercentiles []float64) (types.EthFeeHistory, error) //perm:read

	EthMaxPriorityFeePerGas(ctx context.Context) (types.EthBigInt, error)                   //perm:read
	EthEstimateGas(ctx context.Context, tx types.EthCall) (types.EthUint64, error)          //perm:read
	EthCall(ctx context.Context, tx types.EthCall, blkParam string) (types.EthBytes, error) //perm:read

	EthSendRawTransaction(ctx context.Context, rawTx types.EthBytes) (types.EthHash, error) //perm:read
}
