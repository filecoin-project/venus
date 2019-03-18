package series

import (
	"context"

	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// InitAndStart is a quick way to run Init and Start for a filecoin process.
func InitAndStart(ctx context.Context, node *fast.Filecoin) error {
	if _, err := node.InitDaemon(ctx); err != nil {
		return err
	}

	if _, err := node.StartDaemon(ctx, true); err != nil {
		return err
	}

	return nil
}
