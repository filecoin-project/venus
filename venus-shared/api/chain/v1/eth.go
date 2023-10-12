package v1

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
)

type FullETH interface {
	IETH
	IETHEvent
}

type IETH interface {
	// These methods are used for Ethereum-compatible JSON-RPC calls
	//
	// EthAccounts will always return [] since we don't expect Lotus to manage private keys
	EthAccounts(ctx context.Context) ([]types.EthAddress, error) //perm:read
	// EthAddressToFilecoinAddress converts an EthAddress into an f410 Filecoin Address
	EthAddressToFilecoinAddress(ctx context.Context, ethAddress types.EthAddress) (address.Address, error) //perm:read
	// FilecoinAddressToEthAddress converts an f410 or f0 Filecoin Address to an EthAddress
	FilecoinAddressToEthAddress(ctx context.Context, filecoinAddress address.Address) (types.EthAddress, error) //perm:read
	// EthBlockNumber returns the height of the latest (heaviest) TipSet
	EthBlockNumber(ctx context.Context) (types.EthUint64, error) //perm:read
	// EthGetBlockTransactionCountByNumber returns the number of messages in the TipSet
	EthGetBlockTransactionCountByNumber(ctx context.Context, blkNum types.EthUint64) (types.EthUint64, error) //perm:read
	// EthGetBlockTransactionCountByHash returns the number of messages in the TipSet
	EthGetBlockTransactionCountByHash(ctx context.Context, blkHash types.EthHash) (types.EthUint64, error) //perm:read

	EthGetBlockByHash(ctx context.Context, blkHash types.EthHash, fullTxInfo bool) (types.EthBlock, error)                             //perm:read
	EthGetBlockByNumber(ctx context.Context, blkNum string, fullTxInfo bool) (types.EthBlock, error)                                   //perm:read
	EthGetTransactionByHash(ctx context.Context, txHash *types.EthHash) (*types.EthTx, error)                                          //perm:read
	EthGetTransactionByHashLimited(ctx context.Context, txHash *types.EthHash, limit abi.ChainEpoch) (*types.EthTx, error)             //perm:read
	EthGetTransactionHashByCid(ctx context.Context, cid cid.Cid) (*types.EthHash, error)                                               //perm:read
	EthGetMessageCidByTransactionHash(ctx context.Context, txHash *types.EthHash) (*cid.Cid, error)                                    //perm:read
	EthGetTransactionCount(ctx context.Context, sender types.EthAddress, blkParam types.EthBlockNumberOrHash) (types.EthUint64, error) //perm:read
	EthGetTransactionReceipt(ctx context.Context, txHash types.EthHash) (*types.EthTxReceipt, error)                                   //perm:read
	EthGetTransactionReceiptLimited(ctx context.Context, txHash types.EthHash, limit abi.ChainEpoch) (*types.EthTxReceipt, error)      //perm:read
	EthGetTransactionByBlockHashAndIndex(ctx context.Context, blkHash types.EthHash, txIndex types.EthUint64) (types.EthTx, error)     //perm:read
	EthGetTransactionByBlockNumberAndIndex(ctx context.Context, blkNum types.EthUint64, txIndex types.EthUint64) (types.EthTx, error)  //perm:read

	EthGetCode(ctx context.Context, address types.EthAddress, blkParam types.EthBlockNumberOrHash) (types.EthBytes, error)                               //perm:read
	EthGetStorageAt(ctx context.Context, address types.EthAddress, position types.EthBytes, blkParam types.EthBlockNumberOrHash) (types.EthBytes, error) //perm:read
	EthGetBalance(ctx context.Context, address types.EthAddress, blkParam types.EthBlockNumberOrHash) (types.EthBigInt, error)                           //perm:read
	EthChainId(ctx context.Context) (types.EthUint64, error)                                                                                             //perm:read
	EthSyncing(ctx context.Context) (types.EthSyncingResult, error)                                                                                      //perm:read
	NetVersion(ctx context.Context) (string, error)                                                                                                      //perm:read
	NetListening(ctx context.Context) (bool, error)                                                                                                      //perm:read
	EthProtocolVersion(ctx context.Context) (types.EthUint64, error)                                                                                     //perm:read
	EthGasPrice(ctx context.Context) (types.EthBigInt, error)                                                                                            //perm:read
	EthFeeHistory(ctx context.Context, p jsonrpc.RawParams) (types.EthFeeHistory, error)                                                                 //perm:read

	EthMaxPriorityFeePerGas(ctx context.Context) (types.EthBigInt, error)                                       //perm:read
	EthEstimateGas(ctx context.Context, tx types.EthCall) (types.EthUint64, error)                              //perm:read
	EthCall(ctx context.Context, tx types.EthCall, blkParam types.EthBlockNumberOrHash) (types.EthBytes, error) //perm:read

	EthSendRawTransaction(ctx context.Context, rawTx types.EthBytes) (types.EthHash, error) //perm:read

	// Returns the client version
	Web3ClientVersion(ctx context.Context) (string, error) //perm:read

	// TraceAPI related methods
	//
	// Returns traces created at given block
	EthTraceBlock(ctx context.Context, blkNum string) ([]*types.EthTraceBlock, error) //perm:read
	// Replays all transactions in a block returning the requested traces for each transaction
	EthTraceReplayBlockTransactions(ctx context.Context, blkNum string, traceTypes []string) ([]*types.EthTraceReplayBlockTransaction, error) //perm:read
}

type IETHEvent interface {
	// Returns event logs matching given filter spec.
	EthGetLogs(ctx context.Context, filter *types.EthFilterSpec) (*types.EthFilterResult, error) //perm:read

	// Polling method for a filter, returns event logs which occurred since last poll.
	// (requires write perm since timestamp of last filter execution will be written)
	EthGetFilterChanges(ctx context.Context, id types.EthFilterID) (*types.EthFilterResult, error) //perm:read

	// Returns event logs matching filter with given id.
	// (requires write perm since timestamp of last filter execution will be written)
	EthGetFilterLogs(ctx context.Context, id types.EthFilterID) (*types.EthFilterResult, error) //perm:read

	// Installs a persistent filter based on given filter spec.
	EthNewFilter(ctx context.Context, filter *types.EthFilterSpec) (types.EthFilterID, error) //perm:read

	// Installs a persistent filter to notify when a new block arrives.
	EthNewBlockFilter(ctx context.Context) (types.EthFilterID, error) //perm:read

	// Installs a persistent filter to notify when new messages arrive in the message pool.
	EthNewPendingTransactionFilter(ctx context.Context) (types.EthFilterID, error) //perm:read

	// Uninstalls a filter with given id.
	EthUninstallFilter(ctx context.Context, id types.EthFilterID) (bool, error) //perm:read

	// Subscribe to different event types using websockets
	// eventTypes is one or more of:
	//  - newHeads: notify when new blocks arrive.
	//  - pendingTransactions: notify when new messages arrive in the message pool.
	//  - logs: notify new event logs that match a criteria
	// params contains additional parameters used with the log event type
	// The client will receive a stream of EthSubscriptionResponse values until EthUnsubscribe is called.
	EthSubscribe(ctx context.Context, params jsonrpc.RawParams) (types.EthSubscriptionID, error) //perm:read

	// Unsubscribe from a websocket subscription
	EthUnsubscribe(ctx context.Context, id types.EthSubscriptionID) (bool, error) //perm:read
}

// reverse interface to the client, called after EthSubscribe
type EthSubscriber interface {
	// note: the parameter is types.EthSubscriptionResponse serialized as json object
	EthSubscription(ctx context.Context, params jsonrpc.RawParams) error //rpc_method:eth_subscription notify:true
}

// todo: generate by venus-devtool
type EthSubscriberStruct struct {
	EthSubscriberMethods
}

type EthSubscriberMethods struct {
	EthSubscription func(p0 context.Context, p1 jsonrpc.RawParams) error `notify:"true" rpc_method:"eth_subscription"`
}
