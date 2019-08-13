package series

import (
	"context"

	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// InitAndStart is a quick way to run Init and Start for a filecoin process. A variadic set of functions
// can be passed to run between init and the start of the daemon to make configuration changes.
func InitAndStart(ctx context.Context, node *fast.Filecoin, fns ...func(context.Context, *fast.Filecoin) error) error {
	if _, err := node.InitDaemon(ctx); err != nil {
		return err
	}

	for _, fn := range fns {
		if err := fn(ctx, node); err != nil {
			return err
		}
	}

	if _, err := node.StartDaemon(ctx, true); err != nil {
		return err
	}

	return nil
}
