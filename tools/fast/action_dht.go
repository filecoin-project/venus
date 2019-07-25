package fast

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/multiformats/go-multiaddr"
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
func (f *Filecoin) DHTFindProvs(ctx context.Context, key cid.Cid) ([]routing.QueryEvent, error) {
	var out []routing.QueryEvent

	args := []string{"go-filecoin", "swarm", "findprovs", key.String()}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return out, nil
}
