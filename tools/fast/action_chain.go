package fast

import (
	"context"
	"encoding/json"

	"github.com/ipfs/go-cid"
)

// ChainHead runs the chain head command against the filecoin process.
func (f *Filecoin) ChainHead(ctx context.Context) ([]cid.Cid, error) {
	var out []cid.Cid
	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "venus", "chain", "head"); err != nil {
		return nil, err
	}
	return out, nil

}

// ChainLs runs the chain ls command against the filecoin process.
func (f *Filecoin) ChainLs(ctx context.Context) (*json.Decoder, error) {
	return f.RunCmdLDJSONWithStdin(ctx, nil, "venus", "chain", "ls")
}
