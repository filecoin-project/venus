package commands

import (
	"fmt"
	"strconv"

	"github.com/filecoin-project/go-filecoin/types"
)

func optionalSectorSizeWithDefault(o interface{}, def *types.BytesAmount) (*types.BytesAmount, error) {
	if o != nil {
		n, err := strconv.ParseUint(o.(string), 10, 64)
		if err != nil || n == 0 {
			return nil, fmt.Errorf("invalid sector size: %s", o.(string))
		}

		return types.NewBytesAmount(n), nil
	}

	return def, nil
}
