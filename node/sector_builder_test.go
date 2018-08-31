package node

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tempSectorDirs struct {
	stagingPath string
	sealedPath  string
}

var _ SectorDirs = &tempSectorDirs{}

func newTempSectorDirs() *tempSectorDirs {
	return &tempSectorDirs{
		stagingPath: randTempDir(),
		sealedPath:  randTempDir(),
	}
}

func (f *tempSectorDirs) StagingDir() string {
	return f.stagingPath
}

func (f *tempSectorDirs) SealedDir() string {
	return f.sealedPath
}

func (f *tempSectorDirs) remove() {
	if err := os.RemoveAll(f.sealedPath); err != nil {
		panic(err)
	}

	if err := os.RemoveAll(f.stagingPath); err != nil {
		panic(err)
	}
}

var testSectorSize = 64

// randTempDir creates a subdirectory of os.TempDir() with a
// randomized name and returns the directory's path
func randTempDir() string {
	// create a random string
	b := make([]byte, 20)
	_, err := rand.Read(b)
	if err != nil {
		panic("couldn't read")
	}
	encoded := base32.StdEncoding.EncodeToString(b)

	// append string to temp dir path
	path := filepath.Join(os.TempDir(), encoded)

	// create the directory
	err = os.MkdirAll(path, os.ModePerm)
	if err != nil {
		panic(fmt.Sprintf("couldn't create temp dir %s", path))
	}

	return path
}

func TestSectorBuilderSimple(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dirs, _, sb, _ := nodeWithSectorBuilder(t, testSectorSize)
	defer dirs.remove()

	sector, err := sb.NewSector()
	require.NoError(err)
	ctx := context.Background()

	d1Data := []byte("hello world")
	d1 := &PieceInfo{
		DealID: 5,
		Size:   uint64(len(d1Data)),
	}

	if err := sector.WritePiece(ctx, d1, bytes.NewReader(d1Data)); err != nil {
		t.Fatal(err)
	}

	ag := address.NewForTestGetter()
	_, err = sb.Seal(ctx, sector, ag())
	if err != nil {
		t.Fatal(err)
	}
}

func nodeWithSectorBuilder(t *testing.T, sectorSize int) (*tempSectorDirs, *Node, *SectorBuilder, address.Address) {
	t.Helper()
	require := require.New(t)
	ctx := context.Background()

	nd := MakeOfflineNode(t)

	owner, err := nd.NewAddress()
	require.NoError(err)

	defaultAddr, err := nd.DefaultSenderAddress()
	require.NoError(err)

	tif := th.MakeGenesisFunc(
		th.ActorAccount(owner, types.NewAttoFILFromFIL(1000000)),
		th.ActorAccount(defaultAddr, types.NewAttoFILFromFIL(1000000)),
	)
	require.NoError(nd.ChainMgr.Genesis(ctx, tif))
	require.NoError(nd.Start(ctx))

	pledge := *types.NewBytesAmount(100000)
	coll := *types.NewAttoFILFromFIL(100)

	result := <-RunCreateMiner(t, nd, owner, pledge, core.RequireRandomPeerID(), coll)
	require.NoError(result.Err)
	require.NotNil(result.MinerAddress)

	dirs := newTempSectorDirs()

	sb, err := InitSectorBuilder(nd, *result.MinerAddress, sectorSize, dirs)
	require.NoError(err)

	return dirs, nd, sb, *result.MinerAddress
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
	t.Skip("TODO: enable this test once we're doing the file writing and padding in Rust")

	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()

	dirs, nd, sb, _ := nodeWithSectorBuilder(t, testSectorSize)
	defer dirs.remove()

	var sealingWg sync.WaitGroup
	var sealingErr error
	sealingWg.Add(1)

	sector := sb.curSector

	sb.OnCommitmentAddedToMempool = func(ss *SealedSector, msgCid *cid.Cid, err error) {
		if err != nil {
			panic(err)
		} else if ss != nil && ss.sectorLabel == sector.Label {
			sealingWg.Done()
		}
	}

	requireAddPiece := func(s string) {
		err := sb.AddPiece(ctx, requirePieceInfo(require, nd, []byte(s)))
		assert.NoError(err)
	}

	metadataMustMatch(require, sb, sb.curSector, 0)

	// New access is in the right places.
	stagingAccess1, err := sb.sectorStore.NewStagingSectorAccess()
	require.NoError(err)

	sealedAccess1, err := sb.sectorStore.NewSealedSectorAccess()
	require.NoError(err)

	// New access is generated each time.
	stagingAccess2, err := sb.sectorStore.NewStagingSectorAccess()
	require.NoError(err)

	sealedAccess2, err := sb.sectorStore.NewSealedSectorAccess()
	require.NoError(err)

	assert.NotEqual(stagingAccess1, stagingAccess2)
	assert.NotEqual(sealedAccess1, sealedAccess2)

	metadataMustMatch(require, sb, sb.curSector, 0)
	text := "What's our vector, sector?" // len(text) = 26
	requireAddPiece(text)
	assert.Equal(sector, sb.curSector)
	all := text

	metadataMustMatch(require, sb, sector, 1)

	d := requireReadAll(require, sector)
	assert.Equal(all, string(d))
	assert.Nil(sector.sealed)

	text2 := "We have clearance, Clarence." // len(text2) = 28
	requireAddPiece(text2)
	assert.Equal(sector, sb.curSector)
	all += text2

	d2 := requireReadAll(require, sector)
	assert.Equal(all, string(d2))
	assert.Nil(sector.sealed)

	// persisted and calculated metadata match.
	metadataMustMatch(require, sb, sector, 2)

	text3 := "I'm too sexy for this sector." // len(text3) = 29
	requireAddPiece(text3)

	// wait for sector sealing to complete
	sealingWg.Wait()
	require.NoError(sealingErr)

	assert.NotEqual(sector, sb.curSector)

	// persisted and calculated metadata match after a sector is sealed.
	metadataMustMatch(require, sb, sector, 2)

	newSector := sb.curSector
	d4 := requireReadAll(require, newSector)
	metadataMustMatch(require, sb, newSector, 1)

	assert.Equal(text3, d4)
	sealed := sector.sealed
	assert.NotNil(sealed)
	assert.Nil(newSector.sealed)

	assert.Equal(sealed.sectorLabel, sector.Label)
	assert.Equal(sealed.pieces, sector.Pieces)
	assert.Equal(sealed.size, sector.Size)
	_, err = ioutil.ReadFile(sealed.filename)

	assert.NoError(err)

	meta := sb.curSector.SectorMetadata()
	assert.Len(meta.Pieces, 1)
	assert.Equal(uint64(testSectorSize), meta.Size)
	assert.Equal(testSectorSize-len(text3), int(meta.Free))

	text4 := "I am text, and I am long. My reach exceeds my grasp exceeds exceeds my allotted space."
	err = sb.AddPiece(ctx, requirePieceInfo(require, nd, []byte(text4)))
	assert.EqualError(err, ErrPieceTooLarge.Error())
}

func TestSectorBuilderMetadata(t *testing.T) {
	t.Run("creating datastore keys", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

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
		assert.Contains(k2, commRString(merkleRoot))
	})

	t.Run("sealing sector moves metadata", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		ctx := context.Background()

		dirs, nd, sb, _ := nodeWithSectorBuilder(t, sectorSize)
		defer dirs.remove()

		var sealingWg sync.WaitGroup
		var sealingErr error
		sealingWg.Add(1)

		bytesA := make([]byte, 10+(sectorSize/2))
		bytesB := make([]byte, (sectorSize/2)-10)

		sector := sb.curSector

		sb.OnCommitmentAddedToMempool = func(ss *SealedSector, msgCid *cid.Cid, err error) {
			if ss.sectorLabel == sector.Label {
				sealingErr = err
				sealingWg.Done()
			}
		}

		sb.AddPiece(ctx, requirePieceInfo(require, nd, bytesA))
		sectormeta, err := sb.metadataStore.getSectorMetadata(sector.Label)
		require.NoError(err)
		require.NotNil(sectormeta)

		sb.AddPiece(ctx, requirePieceInfo(require, nd, bytesB))

		// wait for sector sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		_, err = sb.metadataStore.getSectorMetadata(sector.Label)
		require.Error(err)
		require.Contains(err.Error(), "not found")

		sealedmeta, err := sb.metadataStore.getSealedSectorMetadata(sector.sealed.commR)
		require.NoError(err)
		require.NotNil(sealedmeta)

		require.Equal(sector.Size, sealedmeta.Size)
		require.Equal(len(sector.Pieces), len(sealedmeta.Pieces))
		for i := 0; i < len(sector.Pieces); i++ {
			pieceInfoMustEqual(t, sector.Pieces[i], sealedmeta.Pieces[i])
		}
	})
}

func TestSectorStore(t *testing.T) {
	t.Run("it loads a persisted sector", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		ctx := context.Background()

		bytesA := make([]byte, 10+(sectorSize/2))

		dirs, nd, sb, _ := nodeWithSectorBuilder(t, sectorSize)
		defer dirs.remove()

		sector := sb.curSector

		sb.AddPiece(ctx, requirePieceInfo(require, nd, bytesA))

		loaded, err := sb.metadataStore.getSector(sector.Label)
		require.NoError(err)

		sectorsMustEqual(t, sector, loaded)
	})

	t.Run("it loads a persisted, sealed sector", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		ctx := context.Background()

		var sealingWg sync.WaitGroup
		var sealingErr error
		sealingWg.Add(1)

		bytesA := make([]byte, 10+(sectorSize/2))
		bytesB := make([]byte, (sectorSize/2)-10)

		dirs, nd, sb, _ := nodeWithSectorBuilder(t, sectorSize)
		defer dirs.remove()

		sector := sb.curSector

		sb.OnCommitmentAddedToMempool = func(ss *SealedSector, msgCid *cid.Cid, err error) {
			if ss.sectorLabel == sector.Label {
				sealingErr = err
				sealingWg.Done()
			}
		}

		sb.AddPiece(ctx, requirePieceInfo(require, nd, bytesA))
		sb.AddPiece(ctx, requirePieceInfo(require, nd, bytesB))

		// wait for sector sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		require.Equal(1, len(sb.sealedSectors))
		sealedSector := sb.sealedSectors[0]

		loaded, err := sb.metadataStore.getSealedSector(sealedSector.commR)
		require.NoError(err)
		sealedSectorsMustEqual(t, sealedSector, loaded)
	})
}

func TestInitializesSectorBuilderFromPersistedState(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	ctx := context.Background()

	var sealingWg sync.WaitGroup
	var sealingErr error
	sealingWg.Add(1)

	bytesA := make([]byte, 10+(sectorSize/2))
	bytesB := make([]byte, (sectorSize/2)-10)

	dirs, nd, sbA, minerAddr := nodeWithSectorBuilder(t, sectorSize)
	defer dirs.remove()

	sector := sbA.curSector

	sbA.OnCommitmentAddedToMempool = func(ss *SealedSector, msgCid *cid.Cid, err error) {
		if ss.sectorLabel == sector.Label {
			sealingErr = err
			sealingWg.Done()
		}
	}

	sbA.AddPiece(ctx, requirePieceInfo(require, nd, bytesA))

	// sector builder B should have the same state as sector builder A
	sbB, err := InitSectorBuilder(nd, minerAddr, sectorSize, dirs)
	require.NoError(err)

	// can't compare sectors with Equal(s1, s2) because their "file" fields will differ
	sectorBuildersMustEqual(t, sbA, sbB)

	// trigger sealing by adding a second piece
	sbA.AddPiece(ctx, requirePieceInfo(require, nd, bytesB))

	// wait for sealing to complete
	sealingWg.Wait()
	require.NoError(sealingErr)

	// sector builder C should have the same state as sector builder A
	sbC, err := InitSectorBuilder(nd, minerAddr, sectorSize, dirs)
	require.NoError(err)

	sectorBuildersMustEqual(t, sbA, sbC)
}

func TestTruncatesUnsealedSectorOnDiskIfMismatch(t *testing.T) {
	t.Run("it truncates the file if file size > metadata size", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		ctx := context.Background()

		nd := MakeNodesStarted(t, 1, false, true)[0]

		nd.NewAddress() // TODO: default init make an address
		addr, err := nd.DefaultSenderAddress()
		require.NoError(err)

		dirs := newTempSectorDirs()
		defer dirs.remove()

		sbA, err := InitSectorBuilder(nd, addr, sectorSize, dirs)
		require.NoError(err)

		sbA.AddPiece(ctx, requirePieceInfo(require, nd, make([]byte, 10)))
		sbA.AddPiece(ctx, requirePieceInfo(require, nd, make([]byte, 20)))
		sbA.AddPiece(ctx, requirePieceInfo(require, nd, make([]byte, 50)))

		metaA, err := sbA.metadataStore.getSectorMetadata(sbA.curSector.Label)
		require.NoError(err)

		infoA, err := os.Stat(metaA.Filename)
		require.NoError(err)

		// size of file on disk should match what we've persisted as metadata
		require.Equal(int(metaA.Size-metaA.Free), int(infoA.Size()))

		// perform an out-of-band write to the file (replaces its contents)
		ioutil.WriteFile(metaA.Filename, make([]byte, 90), 0600)

		// initialize a new sector builder (simulates the node restarting)
		sbB, err := InitSectorBuilder(nd, addr, sectorSize, dirs)
		require.NoError(err)

		metaB, err := sbB.metadataStore.getSectorMetadata(sbB.curSector.Label)
		require.NoError(err)

		infoB, err := os.Stat(metaB.Filename)
		require.NoError(err)

		// ensure that the file was truncated to match metadata
		require.Equal(int(metaB.Size-metaB.Free), int(infoB.Size()))
		require.Equal(int(infoA.Size()), int(infoB.Size()))
	})

	t.Run("it truncates the metadata if file size < metadata size", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		ctx := context.Background()

		nd := MakeNodesStarted(t, 1, false, true)[0]

		nd.NewAddress() // TODO: default init make an address
		addr, err := nd.DefaultSenderAddress()
		require.NoError(err)

		dirs := newTempSectorDirs()
		defer dirs.remove()

		// Wait a sec, theres no miner here... how can we init a sector builder?
		sbA, err := InitSectorBuilder(nd, addr, sectorSize, dirs)
		require.NoError(err)

		sbA.AddPiece(ctx, requirePieceInfo(require, nd, make([]byte, 10)))
		sbA.AddPiece(ctx, requirePieceInfo(require, nd, make([]byte, 20)))
		sbA.AddPiece(ctx, requirePieceInfo(require, nd, make([]byte, 50)))

		metaA, err := sbA.metadataStore.getSectorMetadata(sbA.curSector.Label)
		require.NoError(err)

		// truncate the file such that its size < sum(size-of-pieces)
		require.NoError(os.Truncate(metaA.Filename, int64(40)))

		// initialize final sector builder
		sbB, err := InitSectorBuilder(nd, addr, sectorSize, dirs)
		require.NoError(err)

		metaB, err := sbA.metadataStore.getSectorMetadata(sbB.curSector.Label)
		require.NoError(err)

		infoB, err := os.Stat(metaB.Filename)
		require.NoError(err)

		// ensure metadata was truncated
		require.Equal(2, len(metaB.Pieces))
		require.Equal(30, int(metaB.Size-metaB.Free))

		// ensure that the file was truncated to match metadata
		require.Equal(int(metaB.Size-metaB.Free), int(infoB.Size()))
	})
}

func TestProverIdCreation(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	hash := address.Hash([]byte("satoshi"))
	addr := address.NewMainnet(hash)

	id, err := proverID(addr)
	require.NoError(err)

	require.Equal(31, len(id))
}

func metadataMustMatch(require *require.Assertions, sb *SectorBuilder, sector *Sector, pieces int) {
	sealed := sector.sealed
	if sealed != nil {
		sealedMeta := sealed.SealedSectorMetadata()
		sealedMetaPersisted, err := sb.metadataStore.getSealedSectorMetadata(sealed.commR)
		require.NoError(err)
		require.Equal(sealedMeta, sealedMetaPersisted)
	} else {
		meta := sector.SectorMetadata()
		require.Len(meta.Pieces, pieces)

		// persisted and calculated metadata match.
		metaPersisted, err := sb.metadataStore.getSectorMetadata(sector.Label)
		require.NoError(err)
		require.Equal(metaPersisted, meta)
	}

	builderMeta := sb.SectorBuilderMetadata()
	builderMetaPersisted, err := sb.metadataStore.getSectorBuilderMetadata(sb.MinerAddr)
	require.NoError(err)
	require.Equal(builderMeta, builderMetaPersisted)
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

func sectorBuildersMustEqual(t *testing.T, sb1 *SectorBuilder, sb2 *SectorBuilder) {
	require := require.New(t)

	require.Equal(sb1.MinerAddr, sb2.MinerAddr)
	require.Equal(sb1.sectorSize, sb2.sectorSize)

	sectorsMustEqual(t, sb1.curSector, sb2.curSector)

	require.Equal(len(sb1.sealedSectors), len(sb2.sealedSectors))
	for i := 0; i < len(sb1.sealedSectors); i++ {
		sealedSectorsMustEqual(t, sb1.sealedSectors[i], sb2.sealedSectors[i])
	}
}

func sealedSectorsMustEqual(t *testing.T, ss1 *SealedSector, ss2 *SealedSector) {
	require := require.New(t)

	if ss1 == nil && ss2 == nil {
		return
	}

	require.Equal(ss1.filename, ss2.filename)
	require.Equal(ss1.label, ss2.label)
	require.Equal(ss1.sectorLabel, ss2.sectorLabel)
	require.Equal(ss1.size, ss2.size)
	require.True(bytes.Equal(ss1.commR, ss2.commR))

	require.Equal(len(ss1.pieces), len(ss2.pieces))
	for i := 0; i < len(ss1.pieces); i++ {
		pieceInfoMustEqual(t, ss1.pieces[i], ss2.pieces[i])
	}
}

func sectorsMustEqual(t *testing.T, s1 *Sector, s2 *Sector) {
	require := require.New(t)

	require.Equal(s1.filename, s2.filename)
	require.Equal(s1.Free, s2.Free)
	require.Equal(s1.ID, s2.ID)
	require.Equal(s1.Label, s2.Label)
	require.Equal(s1.Size, s2.Size)

	sealedSectorsMustEqual(t, s1.sealed, s2.sealed)

	require.Equal(len(s1.Pieces), len(s2.Pieces))
	for i := 0; i < len(s1.Pieces); i++ {
		pieceInfoMustEqual(t, s1.Pieces[i], s2.Pieces[i])
	}
}

func requireReadAll(require *require.Assertions, sector *Sector) string {
	data, err := ioutil.ReadFile(sector.filename)
	require.NoError(err)

	return string(data)
}
