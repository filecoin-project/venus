package fast

import (
	"context"
	"encoding/json"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
)

// DealsList runs the `deals list` command against the filecoin process
func (f *Filecoin) DealsList(ctx context.Context, options ...ActionOption) (*json.Decoder, error) {
	args := []string{"go-filecoin", "deals", "list"}

	for _, option := range options {
		args = append(args, option()...)
	}

	return f.RunCmdLDJSONWithStdin(ctx, nil, args...)
}

// DealsRedeem runs the `deals redeem` command against the filecoin process.
func (f *Filecoin) DealsRedeem(ctx context.Context, dealCid cid.Cid, options ...ActionOption) (cid.Cid, error) {
	var out commands.RedeemResult
	args := []string{"go-filecoin", "deals", "redeem", dealCid.String()}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return cid.Undef, err
	}

	return out.Cid, nil
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
