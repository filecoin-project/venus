package series

import (
	"context"

	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// Connect issues a `swarm connect` to the `from` node, using the addresses of the `to` node
func Connect(ctx context.Context, from, to *fast.Filecoin) error {
	details, err := to.ID(ctx)
	if err != nil {
		return err
	}

	if _, err := from.SwarmConnect(ctx, details.Addresses...); err != nil {
		return err
	}

	return nil
}
