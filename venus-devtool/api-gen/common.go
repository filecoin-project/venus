package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io/ioutil"
	"path/filepath"
	"reflect"

	"github.com/filecoin-project/venus/venus-devtool/util"
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
				MethodNamespace: "Message",
			},
		},
		util.APIMeta{
			Type: reflect.TypeOf((*wallet.IFullAPI)(nil)).Elem(),
			ParseOpt: util.InterfaceParseOption{
				ImportPath: "github.com/filecoin-project/venus/venus-shared/api/wallet",
				IncludeAll: true,
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
