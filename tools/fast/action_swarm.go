package fast

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/filecoin-project/go-filecoin/internal/pkg/net"
)

// SwarmConnect runs the `swarm connect` command against the filecoin process
func (f *Filecoin) SwarmConnect(ctx context.Context, addrs ...multiaddr.Multiaddr) (peer.ID, error) {
	var out peer.ID

	args := []string{"go-filecoin", "swarm", "connect"}

	for _, addr := range addrs {
		args = append(args, addr.String())
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return peer.ID(""), err
	}

	return out, nil
}

// SwarmPeers runs the `swarm peers` command against the filecoin process
func (f *Filecoin) SwarmPeers(ctx context.Context, options ...ActionOption) ([]net.SwarmConnInfo, error) {
	var out net.SwarmConnInfos

	args := []string{"go-filecoin", "swarm", "peers"}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return out.Peers, nil
}
