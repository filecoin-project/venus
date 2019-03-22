package fast

import (
	"context"

	"gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/net"
)

// SwarmConnect runs the `swarm connect` command against the filecoin process
func (f *Filecoin) SwarmConnect(ctx context.Context, addrs ...multiaddr.Multiaddr) (*net.ConnectionResult, error) {
	var out net.ConnectionResult

	args := []string{"go-filecoin", "swarm", "connect"}

	for _, addr := range addrs {
		args = append(args, addr.String())
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out, nil
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
