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

var _ v1api.ICommon = (*CommonModule)(nil)

type CommonModule struct{} // nolint

func NewCommonModule() *CommonModule {
	return new(CommonModule)
}

func (cm *CommonModule) Version(ctx context.Context) (types.Version, error) {
	return types.Version{
		Version:    constants.UserVersion(),
		APIVersion: chain.FullAPIVersion1,
	}, nil
}

func (cm *CommonModule) API() v1api.ICommon {
	return cm
}

func (cm *CommonModule) V0API() v0api.ICommon {
	return &apiwrapper.WrapperV1ICommon{ICommon: cm}
}
