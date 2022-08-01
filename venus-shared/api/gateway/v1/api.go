package gateway

import (
	"github.com/filecoin-project/venus/venus-shared/api"
)

type IGateway interface {
	IProofEvent
	IWalletEvent
	IMarketEvent

	api.Version
}
