package fast

import (
	"context"
	"encoding/json"

	"github.com/ipfs/go-cid"

	commands "github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
)

// DealsList runs the `deals list` command against the filecoin process
func (f *Filecoin) DealsList(ctx context.Context, options ...ActionOption) (*json.Decoder, error) {
	args := []string{"go-filecoin", "deals", "list"}

	for _, option := range options {
		args = append(args, option()...)
	}

	return f.RunCmdLDJSONWithStdin(ctx, nil, args...)
}

// DealsShow runs the `deals show` command against the filecoin process
func (f *Filecoin) DealsShow(ctx context.Context, propCid cid.Cid) (*commands.DealsShowResult, error) {

	var out commands.DealsShowResult

	err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "deals", "show", propCid.String())
	if err != nil {
		return nil, err
	}
	return &out, nil
}
