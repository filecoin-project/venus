package common

import (
	"context"

	apiwrapper "github.com/filecoin-project/venus/app/submodule/common/v0api"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/api/chain"
	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var _ v1api.ICommon = (*CommonAPI)(nil)

type CommonAPI struct{} // nolint

func NewCommonAPI() *CommonAPI {
	return new(CommonAPI)
}

func (a *CommonAPI) Version(ctx context.Context) (types.Version, error) {
	return types.Version{
		Version:    constants.UserVersion(),
		APIVersion: chain.FullAPIVersion1,
	}, nil
}

func (a *CommonAPI) API() v1api.ICommon {
	return a
}

func (a *CommonAPI) V0API() v0api.ICommon {
	return &apiwrapper.WrapperV1ICommon{ICommon: a}
}
