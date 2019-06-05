package fast

import (
	"context"

	cid "github.com/ipfs/go-cid"
)

// DagGet runs the `dag get` command against the filecoin process
func (f *Filecoin) DagGet(ctx context.Context, ref cid.Cid) (map[string]interface{}, error) {
	var out map[string]interface{}

	sRef := ref.String()

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "dag", "get", sRef); err != nil {
		return nil, err
	}

	return out, nil
}
