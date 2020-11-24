package fast

import (
	"context"
	"github.com/filecoin-project/venus/app/submodule/chain"
)

// Protocol runs the `protocol` command against the filecoin process
func (f *Filecoin) Protocol(ctx context.Context) (*chain.ProtocolParams, error) {
	var out chain.ProtocolParams

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "venus", "protocol"); err != nil {
		return nil, err
	}

	return &out, nil
}
