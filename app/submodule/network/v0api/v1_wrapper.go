package v0api

import (
	"context"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/api"
	apitypes "github.com/filecoin-project/venus/venus-shared/api/chain"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

type WrapperV1INetwork struct {
	v1api.INetwork
}

func (w *WrapperV1INetwork) Version(ctx context.Context) (apitypes.Version, error) {
	ver, err := w.INetwork.Version(ctx)
	if err != nil {
		return apitypes.Version{}, err
	}

	ver.APIVersion = api.Version(constants.FullAPIVersion0)

	return ver, nil
}

var _ v1api.INetwork = &WrapperV1INetwork{}
