package fast

import (
	"context"

	"github.com/filecoin-project/go-filecoin/commands"
)

// ActorLs runs the `actor ls` command against the filecoin process.
func (f *Filecoin) ActorLs(ctx context.Context) (*commands.ActorView, error) {
	var out commands.ActorView
	args := []string{"go-filecoin", "actor", "ls"}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out, nil
}
