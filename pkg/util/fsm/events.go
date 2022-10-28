package fsm

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
)

// Chain a interface used to get chain head and net version
type Chain interface {
	ChainHead(ctx context.Context) (TipSetToken, abi.ChainEpoch, error)
	StateNetworkVersion(ctx context.Context, tok TipSetToken) (network.Version, error)
}

// `curH`-`ts.Height` = `confidence`
type (
	HeightHandler func(ctx context.Context, tok TipSetToken, curH abi.ChainEpoch) error
	RevertHandler func(ctx context.Context, tok TipSetToken) error
)

type Events interface {
	ChainAt(hnd HeightHandler, rev RevertHandler, confidence int, h abi.ChainEpoch) error
}
