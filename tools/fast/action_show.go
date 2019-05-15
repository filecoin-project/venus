package fast

import (
	"context"
	"github.com/ipfs/iptb/testbed/interfaces"

	"github.com/ipfs/go-cid"

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
func (f *Filecoin) ShowDeal(ctx context.Context, ref cid.Cid) (*testbedi.Output, error) {
	sRef := ref.String()

	output, err := f.RunCmdWithStdin(ctx, nil, "go-filecoin", "show", "deal", sRef)
	if err != nil {
		return nil, err
	}
	return &output, nil
}
