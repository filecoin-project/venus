package shared_testutils

import (
	"testing"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	gfm_tut "github.com/filecoin-project/go-fil-markets/shared_testutil"
)

func RequireMakeTestPieceStore(t *testing.T) piecestore.PieceStore{
	pieceStore := gfm_tut.NewTestPieceStore()

	expectedCIDs := gfm_tut.GenerateCids(3)
	expectedPieces := [][]byte{[]byte("applesuace"), []byte("jam"), []byte("apricot")}
	missingCID := gfm_tut.GenerateCids(1)[0]

	pieceStore.ExpectMissingCID(missingCID)
	for i, c := range expectedCIDs {
		pieceStore.ExpectCID(c, piecestore.CIDInfo{
			PieceBlockLocations: []piecestore.PieceBlockLocation{
				{
					PieceCID: expectedPieces[i],
				},
			},
		})
	}
	for i, piece := range expectedPieces {
		pieceStore.ExpectPiece(piece, piecestore.PieceInfo{
			Deals: []piecestore.DealInfo{
				{
					Length: uint64(i+1),
				},
			},
		})
	}
	return pieceStore
}