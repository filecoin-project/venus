package node

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	dag "gx/ipfs/QmNUCLv5fmUBuAcwbkt58NQvMcJgd5FPCYV2yNCXq4Wnd6/go-ipfs/merkledag"

	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sectorDirsForTest = &repo.MemRepo{}
var testSectorSize = 64

func TestSimple(t *testing.T) {
	require := require.New(t)
	nd := MakeOfflineNode(t)
	sb := requireSectorBuilder(require, nd, testSectorSize)
	sector, err := sb.NewSector()
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
	ss, err := sb.Seal(sector, ag(), makeFilecoinParameters(testSectorSize, nodeSize))
	if err != nil {
		t.Fatal(err)
	}

	t.Log(ss.merkleRoot)
}

func requireSectorBuilder(require *require.Assertions, nd *Node, sectorSize int) *SectorBuilder {
	sb, err := NewSectorBuilder(nd, types.MakeTestAddress("bar"), sectorSize, sectorDirsForTest)
	sb.publicParameters = makeFilecoinParameters(sectorSize, nodeSize)
	sb.setup = func(minerKey []byte, parameters *PublicParameters, data []byte) ([]byte, error) {
		reverseBytes(data)
		return data, nil
	}

	require.NoError(err)
	return sb
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

	fname := newSectorLabel()
	assert.Len(fname, 32) // Sanity check, nothing more.

	nd := MakeOfflineNode(t)

	sb := requireSectorBuilder(require, nd, testSectorSize)

	assertMetadataMatch := func(sector *Sector, pieces int) {
		sealed := sector.sealed
		if sealed != nil {
			sealedMeta := sealed.SealedSectorMetadata()
			sealedMetaPersisted, err := sb.GetSealedMeta(sealed.merkleRoot)
			assert.NoError(err)
			assert.Equal(sealedMeta, sealedMetaPersisted)
		} else {
			meta := sector.SectorMetadata()
			assert.Len(meta.Pieces, pieces)

			// persisted and calculated metadata match.
			metaPersisted, err := sb.GetMeta(sector.Label)
			assert.NoError(err)
			assert.Equal(metaPersisted, meta)
		}
	}

	requireAddPiece := func(s string) {
		err := sb.AddPiece(ctx, requirePieceInfo(require, nd, []byte(s)))
		assert.NoError(err)
	}

	assertMetadataMatch(sb.CurSector, 0)

	// New paths are in the right places.
	stagingPath, _ := sb.newSectorPath()
	sealedPath, _ := sb.newSealedSectorPath()
	assert.Contains(stagingPath, sb.stagingDir)
	assert.Contains(sealedPath, sb.sealedDir)

	// New paths are generated each time.
	stpath2, _ := sb.newSectorPath()
	sepath2, _ := sb.newSealedSectorPath()
	assert.NotEqual(stagingPath, stpath2)
	assert.NotEqual(sealedPath, sepath2)

	sector := sb.CurSector
	assert.NotNil(sector.file)
	assert.IsType(&os.File{}, sector.file)

	assertMetadataMatch(sb.CurSector, 0)
	text := "What's our vector, sector?" // len(text) = 26
	requireAddPiece(text)
	assert.Equal(sector, sb.CurSector)
	all := text

	assertMetadataMatch(sector, 1)

	d := requireReadAll(require, sector)
	assert.Equal(all, string(d))
	assert.Nil(sector.sealed)

	text2 := "We have clearance, Clarence." // len(text2) = 28
	requireAddPiece(text2)
	assert.Equal(sector, sb.CurSector)
	all += text2

	d2 := requireReadAll(require, sector)
	assert.Equal(all, string(d2))
	assert.Nil(sector.sealed)

	assert.NotContains(string(sector.data), string(d2)) // Document behavior: data only set at sealing time.

	// persisted and calculated metadata match.
	assertMetadataMatch(sector, 2)

	text3 := "I'm too sexy for this sector." // len(text3) = 29
	requireAddPiece(text3)
	time.Sleep(100 * time.Millisecond) // Wait for sealing to finish: FIXME, don't sleep.
	assert.NotEqual(sector, sb.CurSector)

	// persisted and calculated metadata match after a sector is sealed.
	assertMetadataMatch(sector, 2)

	newSector := sb.CurSector
	d4 := requireReadAll(require, newSector)
	assertMetadataMatch(newSector, 1)

	assert.Equal(text3, d4)
	sealed := sector.sealed
	assert.NotNil(sealed)
	assert.Nil(newSector.sealed)

	assert.Equal(sealed.baseSector, sector)
	sealedData, err := sealed.ReadFile()
	assert.NoError(err)

	// Create a padded copy of 'all' and reverse it -- which is what our bogus setup function does.
	bytes := make([]byte, testSectorSize)
	copy(bytes, all)
	reverseBytes(bytes)

	assert.Equal(string(bytes), string(sealedData))

	meta := sb.CurSector.SectorMetadata()
	assert.Len(meta.Pieces, 1)
	assert.Equal(uint64(testSectorSize), meta.Size)
	assert.Equal(testSectorSize-len(text3), int(meta.Free))

	text4 := "I am text, and I am long. My reach exceeds my grasp exceeds exceeds my allotted space."
	err = sb.AddPiece(ctx, requirePieceInfo(require, nd, []byte(text4)))
	assert.EqualError(err, ErrPieceTooLarge.Error())
}

func TestSectorBuilderMetadata(t *testing.T) {
	assert := assert.New(t)

	fname := newSectorLabel()
	assert.Len(fname, 32) // Sanity check, nothing more.

	label := "SECTORFILENAMEWHATEVER"

	k := metadataKey(label).String()
	// Don't accidentally test Datastore namespacing implementation.
	assert.Contains(k, "sectors")
	assert.Contains(k, "metadata")
	assert.Contains(k, label)

	merkleRoot := ([]byte)("someMerkleRootLOL")
	k2 := sealedMetadataKey(merkleRoot).String()
	// Don't accidentally test Datastore namespacing implementation.
	assert.Contains(k2, "sealedSectors")
	assert.Contains(k2, "metadata")
	assert.Contains(k2, merkleString(merkleRoot))
}

func TestSealingMovesMetadata(t *testing.T) {
	require := require.New(t)

	ctx := context.Background()

	nd := MakeOfflineNode(t)

	bytesA := make([]byte, 10+(sectorSize/2))
	bytesB := make([]byte, (sectorSize/2)-10)

	sb := requireSectorBuilder(require, nd, sectorSize)
	sector := sb.CurSector

	sb.AddPiece(ctx, requirePieceInfo(require, nd, bytesA))
	sectormeta, err := sb.GetMeta(sector.Label)
	require.NoError(err)
	require.NotNil(sectormeta)

	sb.AddPiece(ctx, requirePieceInfo(require, nd, bytesB))
	time.Sleep(100 * time.Millisecond) // Wait for sealing to finish: FIXME, don't sleep.

	sectormeta, err = sb.GetMeta(sector.Label)
	require.Error(err)
	require.Contains(err.Error(), "not found")

	sealedmeta, err := sb.GetSealedMeta(sector.sealed.merkleRoot)
	require.NoError(err)
	require.NotNil(sealedmeta)

	require.Equal(sector.Size, sealedmeta.Size)
	require.Equal(len(sector.Pieces), len(sealedmeta.Pieces))
	for i := 0; i < len(sector.Pieces); i++ {
		pieceInfoMustEqual(t, sector.Pieces[i], sealedmeta.Pieces[i])
	}
}

func pieceInfoMustEqual(t *testing.T, p1 *PieceInfo, p2 *PieceInfo) {
	if p1.Size != p2.Size {
		t.Fatalf("p1.Size(%d) != p2.Size(%d)\n", p1.Size, p2.Size)
	}

	if p1.DealID != p2.DealID {
		t.Fatalf("p1.DealID(%d) != p2.DealID(%d)\n", p1.DealID, p2.DealID)
	}

	if !p1.Ref.Equals(p2.Ref) {
		t.Fatalf("p1.Ref(%s) != p2.Ref(%s)\n", p1.Ref.String(), p2.Ref.String())
	}
}

func requireReadAll(require *require.Assertions, sector *Sector) string {
	data, err := sector.ReadFile()
	require.NoError(err)

	return string(data)
}

func reverseBytes(s []byte) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
