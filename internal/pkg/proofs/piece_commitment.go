package proofs

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-sectorbuilder"
)

// GeneratePieceCommitmentRequest represents a request to generate a piece
// commitment (merkle root) of the data in a provided reader.
type GeneratePieceCommitmentRequest struct {
	PieceReader io.Reader
	PieceSize   *types.BytesAmount
}

// GeneratePieceCommitmentResponse represents the commitment bytes
type GeneratePieceCommitmentResponse struct {
	CommP types.CommP
}

// GeneratePieceCommitment produces a piece commitment for the provided data
// stored at a given piece path.
func GeneratePieceCommitment(req GeneratePieceCommitmentRequest) (res GeneratePieceCommitmentResponse, retErr error) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		retErr = err
		return
	}

	defer func() {
		err := os.Remove(file.Name())
		if err != nil && retErr == nil {
			retErr = err
		}
	}()

	n, err := io.Copy(file, req.PieceReader)
	if err != nil {
		retErr = err
		return
	}

	if !types.NewBytesAmount(uint64(n)).Equal(req.PieceSize) {
		retErr = fmt.Errorf("was unable to write all piece bytes to temp file (wrote %dB, pieceSize %dB)", n, req.PieceSize.Uint64())
		return
	}

	commP, err := sectorbuilder.GeneratePieceCommitment(file, req.PieceSize.Uint64())
	if err != nil {
		retErr = err
		return
	}

	res = GeneratePieceCommitmentResponse{
		CommP: commP,
	}

	return
}
