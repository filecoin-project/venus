package common

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"reflect"

	"github.com/filecoin-project/venus/venus-devtool/util"
	gatewayv0 "github.com/filecoin-project/venus/venus-shared/api/gateway/v0"
	gatewayv1 "github.com/filecoin-project/venus/venus-shared/api/gateway/v1"
	gatewayv2 "github.com/filecoin-project/venus/venus-shared/api/gateway/v2"
	market_client "github.com/filecoin-project/venus/venus-shared/api/market/client"
	marketv0 "github.com/filecoin-project/venus/venus-shared/api/market/v0"
	marketv1 "github.com/filecoin-project/venus/venus-shared/api/market/v1"
	"github.com/filecoin-project/venus/venus-shared/api/messager"
	"github.com/filecoin-project/venus/venus-shared/api/wallet"
)

func init() {
	for _, capi := range util.ChainAPIPairs {
		ApiTargets = append(ApiTargets, capi.Venus)
	}

	ApiTargets = append(ApiTargets,
		util.APIMeta{
			Type: reflect.TypeOf((*messager.IMessager)(nil)).Elem(),
			ParseOpt: util.InterfaceParseOption{
				ImportPath: "github.com/filecoin-project/venus/venus-shared/api/messager",
				IncludeAll: true,
			},
			RPCMeta: util.RPCMeta{
				Version:         0,
				MethodNamespace: "Message",
			},
		},
		util.APIMeta{
			Type: reflect.TypeOf((*wallet.IFullAPI)(nil)).Elem(),
			ParseOpt: util.InterfaceParseOption{
				ImportPath: "github.com/filecoin-project/venus/venus-shared/api/wallet",
				IncludeAll: true,
			},
			RPCMeta: util.RPCMeta{
				Version: 0,
			},
		},
		util.APIMeta{
			Type: reflect.TypeOf((*gatewayv2.IGateway)(nil)).Elem(),
			ParseOpt: util.InterfaceParseOption{
				ImportPath: "github.com/filecoin-project/venus/venus-shared/api/gateway/v2",
				IncludeAll: true,
			},
			RPCMeta: util.RPCMeta{
				Version:         2,
				MethodNamespace: "Gateway",
			},
		},
		util.APIMeta{
			Type: reflect.TypeOf((*gatewayv1.IGateway)(nil)).Elem(),
			ParseOpt: util.InterfaceParseOption{
				ImportPath: "github.com/filecoin-project/venus/venus-shared/api/gateway/v1",
				IncludeAll: true,
			},
			RPCMeta: util.RPCMeta{
				Version:         1,
				MethodNamespace: "Gateway",
			},
		},
		util.APIMeta{
			Type: reflect.TypeOf((*gatewayv0.IGateway)(nil)).Elem(),
			ParseOpt: util.InterfaceParseOption{
				ImportPath: "github.com/filecoin-project/venus/venus-shared/api/gateway/v0",
				IncludeAll: true,
			},
			RPCMeta: util.RPCMeta{
				Version:         0,
				MethodNamespace: "Gateway",
			},
		},
		util.APIMeta{
			Type: reflect.TypeOf((*marketv0.IMarket)(nil)).Elem(),
			ParseOpt: util.InterfaceParseOption{
				ImportPath: "github.com/filecoin-project/venus/venus-shared/api/market/v0",
				IncludeAll: true,
			},
			RPCMeta: util.RPCMeta{
				Version:         0,
				MethodNamespace: "VENUS_MARKET",
			},
		},
		util.APIMeta{
			Type: reflect.TypeOf((*marketv1.IMarket)(nil)).Elem(),
			ParseOpt: util.InterfaceParseOption{
				ImportPath: "github.com/filecoin-project/venus/venus-shared/api/market/v1",
				IncludeAll: true,
			},
			RPCMeta: util.RPCMeta{
				Version:         1,
				MethodNamespace: "VENUS_MARKET",
			},
		},
		util.APIMeta{
			Type: reflect.TypeOf((*market_client.IMarketClient)(nil)).Elem(),
			ParseOpt: util.InterfaceParseOption{
				ImportPath: "github.com/filecoin-project/venus/venus-shared/api/market/client",
				IncludeAll: true,
			},
			RPCMeta: util.RPCMeta{
				Version:         0,
				MethodNamespace: "VENUS_MARKET_CLIENT",
			},
		},
	)
}

var ApiTargets []util.APIMeta

func StructName(ifaceName string) string {
	return ifaceName + "Struct"
}

func OutputSourceFile(location, fname string, buf *bytes.Buffer) error {
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("format source content: %w", err)
	}

	outputFile := filepath.Join(location, fname)
	err = os.WriteFile(outputFile, formatted, 0o644)
	if err != nil {
		return fmt.Errorf("write to output %s: %w", outputFile, err)
	}

	return nil
}
