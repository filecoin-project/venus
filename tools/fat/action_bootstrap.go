package fast

import (
	"context"

	"github.com/filecoin-project/go-filecoin/commands"
)

// BootstrapLs runs the `bootstrap ls` command against the filecoin process.
func (f *Filecoin) BootstrapLs(ctx context.Context) (*commands.BootstrapLsResult, error) {
	var out commands.BootstrapLsResult
	args := []string{"go-filecoin", "bootstrap", "ls"}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out, nil

}
