package fast

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/types"
)

// Protocol runs the `protocol` command against the filecoin process
func (f *Filecoin) Protocol(ctx context.Context) (*types.ProtocolParams, error) {
	var out types.ProtocolParams

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "venus", "protocol"); err != nil {
		return nil, err
	}

	return &out, nil
}
