package chain

import (
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

type chainAPI struct { // nolint: golint
	v1api.IAccount
	v1api.IActor
	v1api.IMinerState
	v1api.IChainInfo
}

var _ v1api.IChain = &chainAPI{}
