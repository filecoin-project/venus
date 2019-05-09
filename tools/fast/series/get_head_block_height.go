package series

import (
	"context"

	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/types"
)

// GetHeadBlockHeight will inspect the chain head and return the height
func GetHeadBlockHeight(ctx context.Context, client *fast.Filecoin) (*types.BlockHeight, error) {
	tipset, err := client.ChainHead(ctx)
	if err != nil {
		return nil, err
	}

	return types.NewBlockHeight(uint64(tipset[0].Height)), nil
}
