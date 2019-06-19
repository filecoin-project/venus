package series

import (
	"context"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/types"
)

// GetHeadBlockHeight will inspect the chain head and return the height
func GetHeadBlockHeight(ctx context.Context, client *fast.Filecoin) (*types.BlockHeight, error) {
	tipset, err := client.ChainHead(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "client.ChainHead failed")
	}

	block, err := client.ShowHeader(ctx, tipset[0])
	if err != nil {
		return nil, errors.Wrap(err, "client.ShowBlock failed")
	}

	return types.NewBlockHeight(uint64(block.Height)), nil
}
