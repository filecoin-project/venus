package node

import (
	"errors"
	"reflect"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus/app/submodule/eth"
	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/api/permission"
	"github.com/ipfs-force-community/metrics/ratelimit"
)

type RPCService interface{}

type RPCBuilder struct {
	namespace   []string
	v0APIStruct []interface{}
	v1APIStruct []interface{}
}

func NewBuilder() *RPCBuilder {
	return &RPCBuilder{}
}

func (builder *RPCBuilder) NameSpace(nameSpaece string) *RPCBuilder {
	builder.namespace = append(builder.namespace, nameSpaece)
	return builder
}

func (builder *RPCBuilder) AddServices(services ...RPCService) error {
	for _, service := range services {
		err := builder.AddService(service)
		if err != nil {
			return err
		}
	}
	return nil
}

var ethSubModuleTyp = reflect.TypeOf(&eth.EthSubModule{}).Elem()

func skipV0API(in interface{}) bool {
	inT := reflect.TypeOf(in)
	if inT.Kind() == reflect.Pointer {
		inT = inT.Elem()
	}

	return inT.AssignableTo(ethSubModuleTyp)
}

func (builder *RPCBuilder) AddV0API(service RPCService) error {
	methodName := "V0API"
	serviceV := reflect.ValueOf(service)
	apiMethod := serviceV.MethodByName(methodName)
	if !apiMethod.IsValid() {
		if skipV0API(service) {
			return nil
		}
		return errors.New("expect V0API function")
	}

	apiImpls := apiMethod.Call([]reflect.Value{})
	for _, apiImpl := range apiImpls {
		rt := reflect.TypeOf(apiImpl)
		rv := reflect.ValueOf(apiImpl)
		if rt.Kind() == reflect.Array {
			apiLen := rv.Len()
			for i := 0; i < apiLen; i++ {
				ele := rv.Index(i)
				if ele.IsValid() {
					builder.v0APIStruct = append(builder.v0APIStruct, apiImpl.Interface())
				}
			}
		} else {
			builder.v0APIStruct = append(builder.v0APIStruct, apiImpl.Interface())
		}
	}

	return nil
}

func (builder *RPCBuilder) AddAPI(service RPCService) error {
	methodName := "API"
	serviceV := reflect.ValueOf(service)
	apiMethod := serviceV.MethodByName(methodName)
	if !apiMethod.IsValid() {
		return errors.New("expect API function")
	}

	apiImpls := apiMethod.Call([]reflect.Value{})
	for _, apiImpl := range apiImpls {
		rt := reflect.TypeOf(apiImpl)
		rv := reflect.ValueOf(apiImpl)
		if rt.Kind() == reflect.Array {
			apiLen := rv.Len()
			for i := 0; i < apiLen; i++ {
				ele := rv.Index(i)
				if ele.IsValid() {
					builder.v1APIStruct = append(builder.v1APIStruct, apiImpl.Interface())
				}
			}
		} else {
			builder.v1APIStruct = append(builder.v1APIStruct, apiImpl.Interface())
		}
	}
	return nil
}

func (builder *RPCBuilder) AddService(service RPCService) error {
	err := builder.AddV0API(service)
	if err != nil {
		return err
	}

	return builder.AddAPI(service)
}

func (builder *RPCBuilder) Build(version string, limiter *ratelimit.RateLimiter) *jsonrpc.RPCServer {
	var server *jsonrpc.RPCServer
	serverOptions := make([]jsonrpc.ServerOption, 0)
	serverOptions = append(serverOptions, jsonrpc.WithProxyBind(jsonrpc.PBMethod))

	switch version {
	case "v0":
		server = jsonrpc.NewServer(serverOptions...)

		var fullNodeV0 v0api.FullNodeStruct
		for _, apiStruct := range builder.v0APIStruct {
			permission.PermissionProxy(apiStruct, &fullNodeV0)
		}

		if limiter != nil {
			var rateLimitAPI v0api.FullNodeStruct
			limiter.WraperLimiter(fullNodeV0, &rateLimitAPI)
			fullNodeV0 = rateLimitAPI
		}

		for _, nameSpace := range builder.namespace {
			server.Register(nameSpace, &fullNodeV0)
		}
	case "v1":
		serverOptions = append(serverOptions, jsonrpc.WithReverseClient[v1api.EthSubscriberMethods](v1api.MethodNamespace))
		server = jsonrpc.NewServer(serverOptions...)

		var fullNode v1api.FullNodeStruct
		for _, apiStruct := range builder.v1APIStruct {
			permission.PermissionProxy(apiStruct, &fullNode)
		}

		if limiter != nil {
			var rateLimitAPI v1api.FullNodeStruct
			limiter.WraperLimiter(fullNode, &rateLimitAPI)
			fullNode = rateLimitAPI
		}

		for _, nameSpace := range builder.namespace {
			server.Register(nameSpace, &fullNode)
		}
	default:
		panic("invalid version: " + version)
	}
	aliasETHAPI(server)

	return server
}

func aliasETHAPI(rpcServer *jsonrpc.RPCServer) {
	// TODO: use reflect to automatically register all the eth aliases
	rpcServer.AliasMethod("eth_accounts", "Filecoin.EthAccounts")
	rpcServer.AliasMethod("eth_blockNumber", "Filecoin.EthBlockNumber")
	rpcServer.AliasMethod("eth_getBlockTransactionCountByNumber", "Filecoin.EthGetBlockTransactionCountByNumber")
	rpcServer.AliasMethod("eth_getBlockTransactionCountByHash", "Filecoin.EthGetBlockTransactionCountByHash")

	rpcServer.AliasMethod("eth_getBlockByHash", "Filecoin.EthGetBlockByHash")
	rpcServer.AliasMethod("eth_getBlockByNumber", "Filecoin.EthGetBlockByNumber")
	rpcServer.AliasMethod("eth_getTransactionByHash", "Filecoin.EthGetTransactionByHash")
	rpcServer.AliasMethod("eth_getTransactionHashByCid", "Filecoin.EthGetTransactionHashByCid")
	rpcServer.AliasMethod("eth_getMessageCidByTransactionHash", "Filecoin.EthGetMessageCidByTransactionHash")
	rpcServer.AliasMethod("eth_getTransactionCount", "Filecoin.EthGetTransactionCount")
	rpcServer.AliasMethod("eth_getTransactionReceipt", "Filecoin.EthGetTransactionReceipt")
	rpcServer.AliasMethod("eth_getTransactionByBlockHashAndIndex", "Filecoin.EthGetTransactionByBlockHashAndIndex")
	rpcServer.AliasMethod("eth_getTransactionByBlockNumberAndIndex", "Filecoin.EthGetTransactionByBlockNumberAndIndex")

	rpcServer.AliasMethod("eth_getCode", "Filecoin.EthGetCode")
	rpcServer.AliasMethod("eth_getStorageAt", "Filecoin.EthGetStorageAt")
	rpcServer.AliasMethod("eth_getBalance", "Filecoin.EthGetBalance")
	rpcServer.AliasMethod("eth_chainId", "Filecoin.EthChainId")
	rpcServer.AliasMethod("eth_syncing", "Filecoin.EthSyncing")
	rpcServer.AliasMethod("eth_feeHistory", "Filecoin.EthFeeHistory")
	rpcServer.AliasMethod("eth_protocolVersion", "Filecoin.EthProtocolVersion")
	rpcServer.AliasMethod("eth_maxPriorityFeePerGas", "Filecoin.EthMaxPriorityFeePerGas")
	rpcServer.AliasMethod("eth_gasPrice", "Filecoin.EthGasPrice")
	rpcServer.AliasMethod("eth_sendRawTransaction", "Filecoin.EthSendRawTransaction")
	rpcServer.AliasMethod("eth_estimateGas", "Filecoin.EthEstimateGas")
	rpcServer.AliasMethod("eth_call", "Filecoin.EthCall")

	rpcServer.AliasMethod("eth_getLogs", "Filecoin.EthGetLogs")
	rpcServer.AliasMethod("eth_getFilterChanges", "Filecoin.EthGetFilterChanges")
	rpcServer.AliasMethod("eth_getFilterLogs", "Filecoin.EthGetFilterLogs")
	rpcServer.AliasMethod("eth_newFilter", "Filecoin.EthNewFilter")
	rpcServer.AliasMethod("eth_newBlockFilter", "Filecoin.EthNewBlockFilter")
	rpcServer.AliasMethod("eth_newPendingTransactionFilter", "Filecoin.EthNewPendingTransactionFilter")
	rpcServer.AliasMethod("eth_uninstallFilter", "Filecoin.EthUninstallFilter")
	rpcServer.AliasMethod("eth_subscribe", "Filecoin.EthSubscribe")
	rpcServer.AliasMethod("eth_unsubscribe", "Filecoin.EthUnsubscribe")

	rpcServer.AliasMethod("trace_block", "Filecoin.EthTraceBlock")
	rpcServer.AliasMethod("trace_replayBlockTransactions", "Filecoin.EthTraceReplayBlockTransactions")

	rpcServer.AliasMethod("net_version", "Filecoin.NetVersion")
	rpcServer.AliasMethod("net_listening", "Filecoin.NetListening")

	rpcServer.AliasMethod("web3_clientVersion", "Filecoin.Web3ClientVersion")
}
