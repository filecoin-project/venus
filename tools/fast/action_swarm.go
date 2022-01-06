package fast

import (
	"context"
	"encoding/json"
	"io"

	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/venus/cmd"
	"github.com/ipfs/go-cid"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

// ID runs the `id` command against the filecoin process
func (f *Filecoin) ID(ctx context.Context, options ...ActionOption) (*cmd.IDDetails, error) {
	var out cmd.IDDetails
	args := []string{"venus", "id"}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out, nil
}

// SwarmConnect runs the `swarm connect` command against the filecoin process
func (f *Filecoin) SwarmConnect(ctx context.Context, addrs ...multiaddr.Multiaddr) (peer.ID, error) {
	var out peer.ID

	args := []string{"venus", "swarm", "connect"}

	for _, addr := range addrs {
		args = append(args, addr.String())
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return peer.ID(""), err
	}

	return out, nil
}

// SwarmPeers runs the `swarm peers` command against the filecoin process
func (f *Filecoin) SwarmPeers(ctx context.Context, options ...ActionOption) ([]types.SwarmConnInfo, error) {
	var out types.SwarmConnInfos

	args := []string{"venus", "swarm", "peers"}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return out.Peers, nil
}

// DHTFindPeer runs the `dht findpeer` command against the filecoin process
func (f *Filecoin) DHTFindPeer(ctx context.Context, pid peer.ID) ([]multiaddr.Multiaddr, error) {
	decoder, err := f.RunCmdLDJSONWithStdin(ctx, nil, "venus", "swarm", "findpeer", pid.String())
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
	args := []string{"venus", "dht", "findprovs", key.String()}
	return f.RunCmdLDJSONWithStdin(ctx, nil, args...)

}
