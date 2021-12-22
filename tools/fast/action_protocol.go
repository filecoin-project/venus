package fast

import (
	"context"

	apitypes "github.com/filecoin-project/venus/venus-shared/api/chain"
)

// Protocol runs the `protocol` command against the filecoin process
func (f *Filecoin) Protocol(ctx context.Context) (*apitypes.ProtocolParams, error) {
	var out apitypes.ProtocolParams

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "venus", "protocol"); err != nil {
		return nil, err
	}

	return &out, nil
}
