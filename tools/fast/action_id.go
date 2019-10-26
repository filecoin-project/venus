package fast

import (
	"context"

	"github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
)

// ID runs the `id` command against the filecoin process
func (f *Filecoin) ID(ctx context.Context, options ...ActionOption) (*commands.IDDetails, error) {
	var out commands.IDDetails
	args := []string{"go-filecoin", "id"}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out, nil
}
