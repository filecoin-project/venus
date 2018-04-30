package node

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	dag "github.com/ipfs/go-ipfs/merkledag"

	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sectorDirsForTest = &repo.MemRepo{}

func TestWhatever(t *testing.T) {
	require := require.New(t)
	nd := MakeOfflineNode(t)
	smc := requireSectorBuilder(require, nd, 50)
	sector, err := smc.NewSector()
	require.NoError(err)

	d1Data := []byte("hello world")
	d1 := &PieceInfo{
		DealID: 5,
		Size:   uint64(len(d1Data)),
	}

	if err := sector.WritePiece(d1, bytes.NewReader(d1Data)); err != nil {
		t.Fatal(err)
	}

	ag := types.NewAddressForTestGetter()
	ss, err := smc.Seal(sector, ag(), filecoinParameters)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(ss.merkleRoot)
	t.Log(ss.replicaData)
}

func requireSectorBuilder(require *require.Assertions, nd *Node, sectorSize int) *SectorBuilder {
	smc, err := NewSectorBuilder(nd, sectorSize, sectorDirsForTest)
	require.NoError(err)
	return smc
}

func requirePieceInfo(require *require.Assertions, nd *Node, bytes []byte) *PieceInfo {
	data := dag.NewRawNode(bytes)
	err := nd.Blockservice.AddBlock(data)
	require.NoError(err)
	return &PieceInfo{
		Ref:    data.Cid(),
		Size:   uint64(len(bytes)),
		DealID: 0, // FIXME parameterize
	}
}

func TestSectorBuilder(t *testing.T) {
	defer sectorDirsForTest.CleanupSectorDirs()
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()

	fname := newSectorFileName()
	assert.Len(fname, 32) // Sanity check, nothing more.

	nd := MakeOfflineNode(t)

	smc := requireSectorBuilder(require, nd, 60)

	// New paths are in the right places.
	stagingPath := smc.newSectorPath()
	sealedPath := smc.newSealedSectorPath()
	assert.Contains(stagingPath, smc.stagingDir)
	assert.Contains(sealedPath, smc.sealedDir)

	// New paths are generated each time.
	stpath2 := smc.newSectorPath()
	sepath2 := smc.newSealedSectorPath()
	assert.NotEqual(stagingPath, stpath2)
	assert.NotEqual(sealedPath, sepath2)

	sector := smc.CurSector
	assert.NotNil(sector.file)
	assert.IsType(&os.File{}, sector.file)

	requireAddPiece := func(s string) {
		err := smc.AddPiece(ctx, requirePieceInfo(require, nd, []byte(s)))
		assert.NoError(err)

	}

	text := "What's our vector, sector?" // len(text) = 26
	requireAddPiece(text)
	assert.Equal(sector, smc.CurSector)
	all := text

	d := requireReadAll(require, sector)
	assert.Equal(all, string(d))
	assert.Nil(sector.sealed)

	text2 := "We have clearance, Clarence." // len(text2) = 28
	requireAddPiece(text2)
	assert.Equal(sector, smc.CurSector)
	all += text2

	d2 := requireReadAll(require, sector)
	assert.Equal(all, string(d2))
	assert.Nil(sector.sealed)

	assert.NotContains(string(sector.data), string(d2)) // Document behavior: data only set at sealing time.

	text3 := "I'm too sexy for this sector." // len(text3) = 29
	requireAddPiece(text3)
	time.Sleep(100 * time.Millisecond) // Wait for sealing to finish: FIXME, don't sleep.
	assert.NotEqual(sector, smc.CurSector)

	d3 := requireReadAll(require, sector)
	assert.Equal(all, string(d3)) // Initial sector still contains initial data.

	assert.Contains(string(sector.data), string(d3)) // Sector data has been set. 'Contains' because it is padded.

	newSector := smc.CurSector
	d4 := requireReadAll(require, newSector)

	assert.Equal(text3, d4)
	sealed := sector.sealed
	assert.NotNil(sealed)
	assert.Nil(newSector.sealed)

	assert.Equal(sealed.baseSector, sector)
	sealedData, err := sealed.ReadFile()
	assert.NoError(err)
	assert.Equal(sealed.replicaData, sealedData)

	text4 := "I am text, and I am long. My reach exceeds my grasp exceeds exceeds my allotted space."
	err = smc.AddPiece(ctx, requirePieceInfo(require, nd, []byte(text4)))
	assert.EqualError(err, ErrPieceTooLarge.Error())
}

func requireReadAll(require *require.Assertions, sector *Sector) string {
	data, err := sector.ReadFile()
	require.NoError(err)

	return string(data)
}
