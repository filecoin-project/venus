package v0api

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/venus/venus-shared/api"
	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

type WrapperV1INetwork struct {
	v1api.INetwork
}

func (w *WrapperV1INetwork) Version(ctx context.Context) (types.Version, error) {
	ver, err := w.INetwork.Version(ctx)
	if err != nil {
		return types.Version{}, err
	}

	ver.APIVersion = api.FullAPIVersion0

	return ver, nil
}

var _ v0api.INetwork = &WrapperV1INetwork{}
