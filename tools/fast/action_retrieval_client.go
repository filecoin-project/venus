package fast

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// RetrievalClientRetrievePiece runs the retrieval-client retrieve-piece commands against the filecoin process.
func (f *Filecoin) RetrievalClientRetrievePiece(ctx context.Context, pieceCID cid.Cid, minerAddr address.Address) (io.ReadCloser, error) {
	out, err := f.RunCmdWithStdin(ctx, nil, "go-filecoin", "retrieval-client", "retrieve-piece", minerAddr.String(), pieceCID.String())
	if err != nil {
		return nil, err
	}
	if out.ExitCode() > 0 {
		return nil, fmt.Errorf("filecoin command: %s, exited with non-zero exitcode: %d", out.Args(), out.ExitCode())
	}
	return out.Stdout(), nil
}
