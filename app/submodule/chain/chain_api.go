package chain

import (
	"github.com/filecoin-project/venus/app/client/apiface"
)

type chainAPI struct { // nolint: golint
	apiface.IAccount
	apiface.IActor
	apiface.IBeacon
	apiface.IMinerState
	apiface.IChainInfo
}

var _ apiface.IChain = &chainAPI{}
