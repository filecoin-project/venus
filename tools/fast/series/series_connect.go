package series

import (
	"context"

	"gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"

	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// Connect issues a `swarm connect` to the `from` node, using the addresses of the `to` node
func Connect(ctx context.Context, from, to *fast.Filecoin) error {
	details, err := to.ID(ctx)
	if err != nil {
		return err
	}

	var addrs []multiaddr.Multiaddr
	for _, addr := range details.Addresses {
		if err != nil {
			return err
		}

		addrs = append(addrs, addr)
	}

	if _, err := from.SwarmConnect(ctx, addrs...); err != nil {
		return err
	}

	return nil
}
