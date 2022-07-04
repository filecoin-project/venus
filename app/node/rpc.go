package node

import (
	"errors"
	"reflect"

	"github.com/filecoin-project/go-jsonrpc"
	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/api/permission"
	"github.com/ipfs-force-community/metrics/ratelimit"
)

type RPCService interface {
}

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
func (builder *RPCBuilder) AddService(service RPCService) error {
	methodName := "V0API"

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
					builder.v0APIStruct = append(builder.v0APIStruct, apiImpl.Interface())
				}
			}
		} else {
			builder.v0APIStruct = append(builder.v0APIStruct, apiImpl.Interface())
		}
	}

	methodName = "API"
	serviceV = reflect.ValueOf(service)
	apiMethod = serviceV.MethodByName(methodName)
	if !apiMethod.IsValid() {
		return errors.New("expect API function")
	}

	apiImpls = apiMethod.Call([]reflect.Value{})

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

func (builder *RPCBuilder) Build(version string, limiter *ratelimit.RateLimiter) *jsonrpc.RPCServer {
	serverOptions := make([]jsonrpc.ServerOption, 0)
	serverOptions = append(serverOptions, jsonrpc.WithProxyBind(jsonrpc.PBMethod))

	server := jsonrpc.NewServer(serverOptions...)
	switch version {
	case "v0":
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

	return server
}
