package series

import (
	"context"

	"github.com/filecoin-project/go-filecoin/tools/fast"
)

func InitAndStart(ctx context.Context, node *fast.Filecoin, gcURI string) error {

	if _, err := node.InitDaemon(ctx, "--genesisfile", gcURI); err != nil {
		return err
	}

	if _, err := node.StartDaemon(ctx, true, "--block-time", "5s"); err != nil {
		return err
	}

	return nil
}
