package fast

import (
	"context"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/porcelain"
)

// Protocol runs the `protocol` command against the filecoin process
func (f *Filecoin) Protocol(ctx context.Context) (*porcelain.ProtocolParams, error) {
	var out porcelain.ProtocolParams

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "venus", "protocol"); err != nil {
		return nil, err
	}

	return &out, nil
}
