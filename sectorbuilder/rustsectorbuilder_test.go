package sectorbuilder

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	bserv "gx/ipfs/QmTfTKeBhTLjSjxXQsjkF2b1DfZmYEMnknGE2y2gX57C6v/go-blockservice"
	offline "gx/ipfs/QmZxjqR9Qgompju73kakSoUj3rbVndAzky3oCDiBNCxPs1/go-ipfs-exchange-offline"
	bstore "gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"

	"github.com/filecoin-project/go-filecoin/repo"

	"github.com/stretchr/testify/require"
)

func TestRustSectorBuilderLifecycle(t *testing.T) {
	memRepo := repo.NewInMemoryRepo()
	blockStore := bstore.NewBlockstore(memRepo.Datastore())
	blockService := bserv.New(blockStore, offline.Exchange(blockStore))

	var proverID [31]byte
	copy(proverID[:], requireRandomBytes(t, 31))

	metadataDir, _ := ioutil.TempDir("", "metadata")
	defer os.RemoveAll(metadataDir)

	stagingDir, _ := ioutil.TempDir("", "staging")
	defer os.RemoveAll(stagingDir)

	sealedDir, _ := ioutil.TempDir("", "sealed")
	defer os.RemoveAll(sealedDir)

	sb, err := NewRustSectorBuilder(blockService, ProofTest, 123, metadataDir, proverID, stagingDir, sealedDir)
	require.NoError(t, err)

	// create an IPLD node from user piece-bytes and add it to block service
	pieceBytes := requireRandomBytes(t, 10)
	require.NoError(t, err)
	pieceAsIpldNode := dag.NewRawNode(pieceBytes)
	numIpldNodeBytes, err := pieceAsIpldNode.Size()
	require.NoError(t, err)
	blockService.AddBlock(pieceAsIpldNode)

	sectorID, err := sb.AddPiece(context.Background(), &PieceInfo{
		Ref:  pieceAsIpldNode.Cid(),
		Size: numIpldNodeBytes,
	})
	require.NoError(t, err)
	require.Equal(t, int(124), int(sectorID))
}
