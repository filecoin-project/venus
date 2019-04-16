package fast

import (
	"context"

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
func (f *Filecoin) ChainLs(ctx context.Context) (DecodeCloser, error) {
	return f.RunCmdLDJSONWithStdin(ctx, nil, "go-filecoin", "chain", "ls")
}
