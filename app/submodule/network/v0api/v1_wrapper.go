package v0api

import (
	"context"

	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	"github.com/filecoin-project/venus/pkg/constants"
)

type WrapperV1INetwork struct {
	apiface.INetwork
}

func (w *WrapperV1INetwork) Version(ctx context.Context) (apitypes.Version, error) {
	ver, err := w.INetwork.Version(ctx)
	if err != nil {
		return apitypes.Version{}, err
	}

	ver.APIVersion = constants.FullAPIVersion0

	return ver, nil
}

var _ apiface.INetwork = &WrapperV1INetwork{}
