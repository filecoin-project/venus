package fast

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

// ShowHeader runs the `show header` command against the filecoin process
func (f *Filecoin) ShowHeader(ctx context.Context, ref cid.Cid) (*types.Block, error) {
	var out types.Block

	sRef := ref.String()

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "show", "header", sRef); err != nil {
		return nil, err
	}

	return &out, nil
}
