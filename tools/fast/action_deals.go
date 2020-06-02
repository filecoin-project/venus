package fast

import (
	"context"
	"encoding/json"
)

// DealsList runs the `deals list` command against the filecoin process
func (f *Filecoin) DealsList(ctx context.Context, options ...ActionOption) (*json.Decoder, error) {
	args := []string{"go-filecoin", "deals", "list"}

	for _, option := range options {
		args = append(args, option()...)
	}

	return f.RunCmdLDJSONWithStdin(ctx, nil, args...)
}
