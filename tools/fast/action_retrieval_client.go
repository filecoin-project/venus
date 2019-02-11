package fast

import (
	"context"
	"io"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
)

// RetrievalClientRetrievePiece runs the retrieval-client retrieve-piece commands against the filecoin process.
func (f *Filecoin) RetrievalClientRetrievePiece(ctx context.Context, pieceCID cid.Cid, minerAddr address.Address) (io.ReadCloser, error) {
	out, err := f.RunCmdWithStdin(ctx, nil, "go-filecoin", "retrieval-client", "retrieve-piece", minerAddr.String(), pieceCID.String())
	if err != nil {
		return nil, err
	}
	return out.Stdout(), nil
}
