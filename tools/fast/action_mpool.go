package fast

import (
	"context"

	types "github.com/filecoin-project/venus/venus-shared/chain"
)

// MpoolLs runs the `mpool ls` command against the filecoin process.
func (f *Filecoin) MpoolLs(ctx context.Context, options ...ActionOption) ([]*types.SignedMessage, error) {
	var out []*types.SignedMessage

	args := []string{"venus", "mpool", "ls"}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return []*types.SignedMessage{}, err
	}

	return out, nil
}
