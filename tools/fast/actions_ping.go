package fast

import (
	"context"
	"io"

	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/porcelain"
)

// Ping runs the `ping` command against the filecoin process
func (f *Filecoin) Ping(ctx context.Context, pid peer.ID, options ...ActionOption) ([]porcelain.PingResult, error) {
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

	var out []porcelain.PingResult

	for {
		var result porcelain.PingResult
		if err := decoder.Decode(&result); err != nil {
			if err == io.EOF {
				break
			}

			return []porcelain.PingResult{}, err
		}

		out = append(out, result)
	}

	return out, nil
}
