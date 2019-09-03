package fast

import (
	"context"
	"encoding/json"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
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
func (f *Filecoin) DHTFindProvs(ctx context.Context, key cid.Cid) (*json.Decoder, error) {
	args := []string{"go-filecoin", "dht", "findprovs", key.String()}
	return f.RunCmdLDJSONWithStdin(ctx, nil, args...)
}
