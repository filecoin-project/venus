package fast

import (
	"context"
	"io"

	"github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
)

// ActorLs runs the `actor ls` command against the filecoin process.
func (f *Filecoin) ActorLs(ctx context.Context) ([]commands.ActorView, error) {
	args := []string{"go-filecoin", "actor", "ls"}

	dec, err := f.RunCmdLDJSONWithStdin(ctx, nil, args...)
	if err != nil {
		return nil, err
	}

	views := []commands.ActorView{}
	for dec.More() {
		var view commands.ActorView
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
