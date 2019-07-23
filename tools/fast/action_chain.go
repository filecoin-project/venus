package fast

import (
	"context"
	"encoding/json"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/ipfs/go-cid"
)

// ChainHead runs the chain head command against the filecoin process.
func (f *Filecoin) ChainHead(ctx context.Context) ([]cid.Cid, error) {
	var out []cid.Cid
	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "chain", "head"); err != nil {
		return nil, err
	}
	return out, nil

}

// ChainLs runs the chain ls command against the filecoin process.
func (f *Filecoin) ChainLs(ctx context.Context) (*json.Decoder, error) {
	return f.RunCmdLDJSONWithStdin(ctx, nil, "go-filecoin", "chain", "ls")
}

// ChainBlock runs the `chain block` command against the filecoin process
func (f *Filecoin) ChainBlock(ctx context.Context, ref cid.Cid) (*types.Block, error) {
	var out types.Block

	sRef := ref.String()

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "chain", "block", sRef); err != nil {
		return nil, err
	}

	return &out, nil
}
