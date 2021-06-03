package node

import (
	"reflect"

	"github.com/filecoin-project/go-jsonrpc"

	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/app/client"
	"github.com/filecoin-project/venus/app/client/funcrule"
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
		return xerrors.New("expect API function")
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
		return xerrors.New("expect API function")
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

func (builder *RPCBuilder) Build(version string) *jsonrpc.RPCServer {
	serverOptions := make([]jsonrpc.ServerOption, 0)
	serverOptions = append(serverOptions, jsonrpc.WithProxyBind(jsonrpc.PBField))

	server := jsonrpc.NewServer(serverOptions...)
	var fullNode client.FullNodeStruct
	switch version {
	case "v0":
		for _, apiStruct := range builder.v0APIStruct {
			funcrule.PermissionProxy(apiStruct, &fullNode)
		}
	case "v1":
		for _, apiStruct := range builder.v1APIStruct {
			funcrule.PermissionProxy(apiStruct, &fullNode)
		}
	default:

	}

	for _, nameSpace := range builder.namespace {
		server.Register(nameSpace, &fullNode)
	}
	return server
}
