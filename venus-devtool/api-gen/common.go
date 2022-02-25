package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io/ioutil"
	"path/filepath"
	"reflect"

	"github.com/filecoin-project/venus/venus-devtool/util"
	gatewayv0 "github.com/filecoin-project/venus/venus-shared/api/gateway/v0"
	gatewayv1 "github.com/filecoin-project/venus/venus-shared/api/gateway/v1"
	"github.com/filecoin-project/venus/venus-shared/api/market"
	market_client "github.com/filecoin-project/venus/venus-shared/api/market/client"
	"github.com/filecoin-project/venus/venus-shared/api/messager"
	"github.com/filecoin-project/venus/venus-shared/api/wallet"
)

func init() {
	for _, capi := range util.ChainAPIPairs {
		apiTargets = append(apiTargets, capi.Venus)
	}

	apiTargets = append(apiTargets,
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
			Type: reflect.TypeOf((*market.IMarket)(nil)).Elem(),
			ParseOpt: util.InterfaceParseOption{
				ImportPath: "github.com/filecoin-project/venus/venus-shared/api/market",
				IncludeAll: true,
			},
			RPCMeta: util.RPCMeta{
				Version:         0,
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

var apiTargets []util.APIMeta

func structName(ifaceName string) string {
	return ifaceName + "Struct"
}

func outputSourceFile(location, fname string, buf *bytes.Buffer) error {
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("format source content: %w", err)
	}

	outputFile := filepath.Join(location, fname)
	err = ioutil.WriteFile(outputFile, formatted, 0644)
	if err != nil {
		return fmt.Errorf("write to output %s: %w", outputFile, err)
	}

	return nil
}
