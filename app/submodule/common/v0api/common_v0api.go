package v0api

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/api/chain"
	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var _ v0api.ICommon = (*WrapperV1ICommon)(nil)

type WrapperV1ICommon struct { //nolint
	v1api.ICommon
}

func (a *WrapperV1ICommon) Version(ctx context.Context) (types.Version, error) {
	ver, err := a.ICommon.Version(ctx)
	if err != nil {
		return types.Version{}, err
	}

	ver.APIVersion = chain.FullAPIVersion0

	return ver, nil
}
