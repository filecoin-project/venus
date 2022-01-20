package util

import (
	"reflect"

	"github.com/filecoin-project/lotus/api/v0api"
	"github.com/filecoin-project/lotus/api/v1api"

	"github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	"github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

var APIPairs = []struct {
	Ver   int
	Lotus APIMeta
	Venus APIMeta
}{
	{
		Ver: 0,
		Lotus: APIMeta{
			Type: reflect.TypeOf((*v0api.FullNode)(nil)).Elem(),
			ParseOpt: InterfaceParseOption{
				ImportPath: "github.com/filecoin-project/lotus/api/v0api",
				Included:   []string{"FullNode", "Common", "Net"},
			},
		},
		Venus: APIMeta{
			Type: reflect.TypeOf((*v0.FullNode)(nil)).Elem(),
			ParseOpt: InterfaceParseOption{
				ImportPath: "github.com/filecoin-project/venus/venus-shared/api/chain/v0",
				IncludeAll: true,
			},
		},
	},
	{
		Ver: 1,
		Lotus: APIMeta{
			Type: reflect.TypeOf((*v1api.FullNode)(nil)).Elem(),
			ParseOpt: InterfaceParseOption{
				ImportPath: "github.com/filecoin-project/lotus/api",
				Included:   []string{"FullNode", "Common", "Net"},
			},
		},
		Venus: APIMeta{
			Type: reflect.TypeOf((*v1.FullNode)(nil)).Elem(),
			ParseOpt: InterfaceParseOption{
				ImportPath: "github.com/filecoin-project/venus/venus-shared/api/chain/v1",
				IncludeAll: true,
			},
		},
	},
}

var LatestAPIPair = APIPairs[len(APIPairs)-1]

type APIMeta struct {
	Type     reflect.Type
	ParseOpt InterfaceParseOption
}
