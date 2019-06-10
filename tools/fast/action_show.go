package fast

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/types"
)

// ShowBlock runs the `show block` command against the filecoin process
func (f *Filecoin) ShowBlock(ctx context.Context, ref cid.Cid) (*types.Block, error) {
	var out types.Block

	sRef := ref.String()

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "show", "block", sRef); err != nil {
		return nil, err
	}

	return &out, nil
}

// ShowDeal runs the `show deal` command against the filecoin process
func (f *Filecoin) ShowDeal(ctx context.Context, propCid cid.Cid) (*storagedeal.Deal, error) {

	var out storagedeal.Deal

	err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "show", "deal", propCid.String())
	if err != nil {
		return nil, err
	}
	return &out, nil
}
