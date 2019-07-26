package fast

import (
	"context"
	"io"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/commands"
)

// Ping runs the `ping` command against the filecoin process
func (f *Filecoin) Ping(ctx context.Context, pid peer.ID, options ...ActionOption) ([]commands.PingResult, error) {
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

	var out []commands.PingResult

	for {
		var result commands.PingResult
		if err := decoder.Decode(&result); err != nil {
			if err == io.EOF {
				break
			}

			return []commands.PingResult{}, err
		}

		out = append(out, result)
	}

	return out, nil
}
