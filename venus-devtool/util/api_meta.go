package util

import (
	"reflect"
	"strings"

	"github.com/filecoin-project/lotus/api/v0api"
	"github.com/filecoin-project/lotus/api/v1api"

	"github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	"github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

var ChainAPIPairs = []struct {
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
			RPCMeta: RPCMeta{
				Version: 0,
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
			RPCMeta: RPCMeta{
				Version: 1,
			},
		},
	},
}

var LatestChainAPIPair = ChainAPIPairs[len(ChainAPIPairs)-1]

type RPCMeta struct {
	Version         uint32
	Namespace       string
	MethodNamespace string
}

type APIMeta struct {
	Type     reflect.Type
	ParseOpt InterfaceParseOption
	RPCMeta
}

func GetAPIMethodPerm(m InterfaceMethodMeta) string {
	permStr := ""

	if cmtNum := len(m.Comments); cmtNum > 0 {
		if itemNum := len(m.Comments[cmtNum-1].List); itemNum > 0 {
			if strings.HasPrefix(m.Comments[cmtNum-1].List[0].Text, "//") {
				permStr = m.Comments[cmtNum-1].List[0].Text[2:]
			}
		}
	}

	for _, piece := range strings.Split(permStr, " ") {
		trimmed := strings.TrimSpace(piece)
		if strings.HasPrefix(trimmed, "perm:") {
			return trimmed[5:]
		}
	}

	return ""
}
