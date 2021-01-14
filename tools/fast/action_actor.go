package fast

import (
	"context"
	"github.com/filecoin-project/venus/cmd"
	"io"
)

// ActorLs runs the `actor ls` command against the filecoin process.
func (f *Filecoin) ActorLs(ctx context.Context) ([]cmd.ActorView, error) {
	args := []string{"venus", "state", "list-actor"}

	dec, err := f.RunCmdLDJSONWithStdin(ctx, nil, args...)
	if err != nil {
		return nil, err
	}

	views := []cmd.ActorView{}
	for dec.More() {
		var view cmd.ActorView
		err := dec.Decode(&view)
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		views = append(views, view)
	}

	return views, nil
}
