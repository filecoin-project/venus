package fast

import (
	"context"
	"io"

	"gx/ipfs/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/api"
)

// Ping runs the `ping` command against the filecoin process
func (f *Filecoin) Ping(ctx context.Context, pid peer.ID, options ...ActionOption) ([]api.PingResult, error) {
	sPid := pid.Pretty()

	args := []string{"go-filecoin", "ping"}

	for _, option := range options {
		args = append(args, option()...)
	}

	args = append(args, sPid)

	decoder, err := f.RunCmdLDJSONWithStdin(ctx, nil, args...)
	if err != nil {
		return nil, err
	}

	var out []api.PingResult

	for {
		var result api.PingResult
		if err := decoder.Decode(&result); err != nil {
			if err == io.EOF {
				break
			}

			return []api.PingResult{}, err
		}

		out = append(out, result)
	}

	return out, nil
}
