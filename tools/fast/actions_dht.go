package fast

import (
	"context"
	"io"

	"gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
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
