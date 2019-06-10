package fast

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/commands"
)

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
