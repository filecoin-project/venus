package fast

import (
	"context"

	"github.com/filecoin-project/go-filecoin/plumbing/actr"
)

// ActorLs runs the `actor ls` command against the filecoin process.
func (f *Filecoin) ActorLs(ctx context.Context) (*actr.ActorView, error) {
	var out actr.ActorView
	args := []string{"go-filecoin", "actor", "ls"}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out, nil
}
