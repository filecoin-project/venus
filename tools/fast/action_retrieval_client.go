package fast

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
)

// RetrievalClientRetrievePiece runs the retrieval-client retrieve-piece commands against the filecoin process.
func (f *Filecoin) RetrievalClientRetrievePiece(ctx context.Context, pieceCID cid.Cid, minerAddr address.Address) (io.ReadCloser, error) {
	out, err := f.RunCmdWithStdin(ctx, nil, "go-filecoin", "retrieval-client", "retrieve-piece", minerAddr.String(), pieceCID.String())
	if err != nil {
		return nil, err
	}

	stdout := out.Stdout()

	rc := &readCloser{
		r: stdout,
		closer: func() error {
			if err := stdout.Close(); err != nil {
				return err
			}

			if err := getOutputError(out); err != nil {
				return err
			}

			return nil
		},
	}

	return rc, nil
}
