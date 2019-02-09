package fast

import (
	"context"
	"github.com/filecoin-project/go-filecoin/api"
)

// ID runs the `id` command against the filecoin process
func (f *Filecoin) ID(ctx context.Context, options ...ActionOption) (*api.IDDetails, error) {
	var out api.IDDetails
	args := []string{"go-filecoin", "id"}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out, nil
}
