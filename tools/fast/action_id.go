package fast

import (
	"context"
	"github.com/filecoin-project/venus/cmd"
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
