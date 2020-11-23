package fast

import (
	"context"
	"github.com/filecoin-project/venus/cmd"
)

// BootstrapLs runs the `bootstrap ls` command against the filecoin process.
func (f *Filecoin) BootstrapLs(ctx context.Context) (*cmd.BootstrapLsResult, error) {
	var out cmd.BootstrapLsResult
	args := []string{"venus", "bootstrap", "ls"}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out, nil

}
