package fast

import (
	"context"
	"io"

	"gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	"gx/ipfs/QmWaDSNoSdSXU9b6udyaq9T8y6LkzMwqWxECznFqvtcTsk/go-libp2p-routing/notifications"
)

// DHTFindPeer runs the `dht findpeer` command against the filecoin process
func (f *Filecoin) DHTFindPeer(ctx context.Context, pid peer.ID) ([]multiaddr.Multiaddr, error) {
	decoder, err := f.RunCmdLDJSONWithStdin(ctx, nil, "go-filecoin", "dht", "findpeer", pid.String())
	if err != nil {
		return nil, err
	}

	var out []multiaddr.Multiaddr
	for {
		var addr string
		if err := decoder.Decode(&addr); err != nil {
			if err == io.EOF {
				break
			}

			return []multiaddr.Multiaddr{}, err
		}

		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return []multiaddr.Multiaddr{}, err
		}

		out = append(out, ma)
	}

	return out, nil
}

// DHTFindProvs runs the `dht findprovs` command against the filecoin process
func (f *Filecoin) DHTFindProvs(ctx context.Context, key cid.Cid) ([]notifications.QueryEvent, error) {
	var out []notifications.QueryEvent

	args := []string{"go-filecoin", "swarm", "findprovs", key.String()}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return out, nil
}
