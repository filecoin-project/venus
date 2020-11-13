package fsmchain

import (
	"context"
	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/internal/pkg/chain"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
	"github.com/filecoin-project/venus/internal/pkg/util/fsm"
)

// ChainConnector uses the chain store to provide a ChainHead method
type ChainConnector struct {
	chainStore *chain.Store
}

var _ fsm.Chain = new(ChainConnector)

func NewChainConnector(chainStore *chain.Store) ChainConnector {
	return ChainConnector{chainStore: chainStore}
}

func (a *ChainConnector) ChainHead(ctx context.Context) (fsm.TipSetToken, abi.ChainEpoch, error) {
	// TODO: use the provided context
	ts, err := a.chainStore.GetTipSet(a.chainStore.GetHead())
	if err != nil {
		return nil, 0, err
	}

	tok, err := encoding.Encode(ts.Key())
	if err != nil {
		return nil, 0, err
	}

	height, err := ts.Height()
	if err != nil {
		return nil, 0, err
	}

	return tok, height, err
}

func (a *ChainConnector) StateNetworkVersion(ctx context.Context, tok fsm.TipSetToken) (network.Version, error) {
	panic("implement me")
}
