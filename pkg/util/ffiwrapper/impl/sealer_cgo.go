//go:build cgo
// +build cgo

package impl

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"os"
	"runtime"

	"github.com/filecoin-project/go-commp-utils/zerocomm"

	"github.com/ipfs/go-cid"

	ffi "github.com/filecoin-project/filecoin-ffi"
	rlepluslazy "github.com/filecoin-project/go-bitfield/rle"
	commpffi "github.com/filecoin-project/go-commp-utils/ffiwrapper"
	commcid "github.com/filecoin-project/go-fil-commcid"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-storage/storage"

	"github.com/filecoin-project/venus/pkg/util/fr32"
	"github.com/filecoin-project/venus/pkg/util/storiface"
)

var _ Storage = &Sealer{}

func New(sectors SectorProvider) (*Sealer, error) {
	sb := &Sealer{
		sectors: sectors,

		stopping: make(chan struct{}),
	}

	return sb, nil
}

func (sb *Sealer) NewSector(ctx context.Context, sector storage.SectorRef) error {
	// TODO: Allocate the sector here instead of in addpiece

	return nil
}

func (sb *Sealer) AddPiece(ctx context.Context, sector storage.SectorRef, existingPieceSizes []abi.UnpaddedPieceSize, pieceSize abi.UnpaddedPieceSize, file storage.Data) (abi.PieceInfo, error) {
	// TODO: allow tuning those:
	chunk := abi.PaddedPieceSize(4 << 20)
	parallel := runtime.NumCPU()

	var offset abi.UnpaddedPieceSize
	for _, size := range existingPieceSizes {
		offset += size
	}

	ssize, err := sector.ProofType.SectorSize()
	if err != nil {
		return abi.PieceInfo{}, err
	}

	maxPieceSize := abi.PaddedPieceSize(ssize)

	if offset.Padded()+pieceSize.Padded() > maxPieceSize {
		return abi.PieceInfo{}, fmt.Errorf("can't add %d byte piece to sector %v with %d bytes of existing pieces", pieceSize, sector, offset)
	}

	var done func()
	var stagedFile *partialFile

	defer func() {
		if done != nil {
			done()
		}

		if stagedFile != nil {
			if err := stagedFile.Close(); err != nil {
				log.Errorf("closing staged file: %+v", err)
			}
		}
	}()

	var stagedPath storiface.SectorPaths
	if len(existingPieceSizes) == 0 {
		stagedPath, done, err = sb.sectors.AcquireSector(ctx, sector, 0, storiface.FTUnsealed, storiface.PathSealing)
		if err != nil {
			return abi.PieceInfo{}, fmt.Errorf("acquire unsealed sector: %w", err)
		}

		stagedFile, err = createPartialFile(maxPieceSize, stagedPath.Unsealed)
		if err != nil {
			return abi.PieceInfo{}, fmt.Errorf("creating unsealed sector file: %w", err)
		}
	} else {
		stagedPath, done, err = sb.sectors.AcquireSector(ctx, sector, storiface.FTUnsealed, 0, storiface.PathSealing)
		if err != nil {
			return abi.PieceInfo{}, fmt.Errorf("acquire unsealed sector: %w", err)
		}

		stagedFile, err = openPartialFile(maxPieceSize, stagedPath.Unsealed)
		if err != nil {
			return abi.PieceInfo{}, fmt.Errorf("opening unsealed sector file: %w", err)
		}
	}

	w, err := stagedFile.Writer(storiface.UnpaddedByteIndex(offset).Padded(), pieceSize.Padded())
	if err != nil {
		return abi.PieceInfo{}, fmt.Errorf("getting partial file writer: %w", err)
	}

	pw := fr32.NewPadWriter(w)

	pr := io.TeeReader(io.LimitReader(file, int64(pieceSize)), pw)

	throttle := make(chan []byte, parallel)
	piecePromises := make([]func() (abi.PieceInfo, error), 0)

	buf := make([]byte, chunk.Unpadded())
	for i := 0; i < parallel; i++ {
		if abi.UnpaddedPieceSize(i)*chunk.Unpadded() >= pieceSize {
			break // won't use this many buffers
		}
		throttle <- make([]byte, chunk.Unpadded())
	}

	for {
		var read int
		for rbuf := buf; len(rbuf) > 0; {
			n, err := pr.Read(rbuf)
			if err != nil && err != io.EOF {
				return abi.PieceInfo{}, fmt.Errorf("pr read error: %w", err)
			}

			rbuf = rbuf[n:]
			read += n

			if err == io.EOF {
				break
			}
		}
		if read == 0 {
			break
		}

		done := make(chan struct {
			cid.Cid
			error
		}, 1)
		pbuf := <-throttle
		copy(pbuf, buf[:read])

		go func(read int) {
			defer func() {
				throttle <- pbuf
			}()

			c, err := sb.pieceCid(sector.ProofType, pbuf[:read])
			done <- struct {
				cid.Cid
				error
			}{c, err}
		}(read)

		piecePromises = append(piecePromises, func() (abi.PieceInfo, error) {
			select {
			case e := <-done:
				if e.error != nil {
					return abi.PieceInfo{}, e.error
				}

				return abi.PieceInfo{
					Size:     abi.UnpaddedPieceSize(len(buf[:read])).Padded(),
					PieceCID: e.Cid,
				}, nil
			case <-ctx.Done():
				return abi.PieceInfo{}, ctx.Err()
			}
		})
	}

	if err := pw.Close(); err != nil {
		return abi.PieceInfo{}, fmt.Errorf("closing padded writer: %w", err)
	}

	if err := stagedFile.MarkAllocated(storiface.UnpaddedByteIndex(offset).Padded(), pieceSize.Padded()); err != nil {
		return abi.PieceInfo{}, fmt.Errorf("marking data range as allocated: %w", err)
	}

	if err := stagedFile.Close(); err != nil {
		return abi.PieceInfo{}, err
	}
	stagedFile = nil

	if len(piecePromises) == 1 {
		return piecePromises[0]()
	}

	pieceCids := make([]abi.PieceInfo, len(piecePromises))
	for i, promise := range piecePromises {
		pieceCids[i], err = promise()
		if err != nil {
			return abi.PieceInfo{}, err
		}
	}

	pieceCID, err := ffi.GenerateUnsealedCID(sector.ProofType, pieceCids)
	if err != nil {
		return abi.PieceInfo{}, fmt.Errorf("generate unsealed CID: %w", err)
	}

	// validate that the pieceCID was properly formed
	if _, err := commcid.CIDToPieceCommitmentV1(pieceCID); err != nil {
		return abi.PieceInfo{}, err
	}

	return abi.PieceInfo{
		Size:     pieceSize.Padded(),
		PieceCID: pieceCID,
	}, nil
}

func (sb *Sealer) pieceCid(spt abi.RegisteredSealProof, in []byte) (cid.Cid, error) {
	prf, werr, err := commpffi.ToReadableFile(bytes.NewReader(in), int64(len(in)))
	if err != nil {
		return cid.Undef, fmt.Errorf("getting tee reader pipe: %w", err)
	}

	pieceCID, err := ffi.GeneratePieceCIDFromFile(spt, prf, abi.UnpaddedPieceSize(len(in)))
	if err != nil {
		return cid.Undef, fmt.Errorf("generating piece commitment: %w", err)
	}

	_ = prf.Close()

	return pieceCID, werr()
}

func (sb *Sealer) SealPreCommit1(ctx context.Context, sector storage.SectorRef, ticket abi.SealRandomness, pieces []abi.PieceInfo) (out storage.PreCommit1Out, err error) {
	paths, done, err := sb.sectors.AcquireSector(ctx, sector, storiface.FTUnsealed, storiface.FTSealed|storiface.FTCache, storiface.PathSealing)
	if err != nil {
		return nil, fmt.Errorf("acquiring sector paths: %w", err)
	}
	defer done()

	e, err := os.OpenFile(paths.Sealed, os.O_RDWR|os.O_CREATE, 0644) // nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("ensuring sealed file exists: %w", err)
	}
	if err := e.Close(); err != nil {
		return nil, err
	}

	if err := os.Mkdir(paths.Cache, 0755); err != nil { // nolint
		if os.IsExist(err) {
			log.Warnf("existing cache in %s; removing", paths.Cache)

			if err := os.RemoveAll(paths.Cache); err != nil {
				return nil, fmt.Errorf("remove existing sector cache from %s (sector %d): %w", paths.Cache, sector, err)
			}

			if err := os.Mkdir(paths.Cache, 0755); err != nil { // nolint:gosec
				return nil, fmt.Errorf("mkdir cache path after cleanup: %w", err)
			}
		} else {
			return nil, err
		}
	}

	var sum abi.UnpaddedPieceSize
	for _, piece := range pieces {
		sum += piece.Size.Unpadded()
	}
	ssize, err := sector.ProofType.SectorSize()
	if err != nil {
		return nil, err
	}
	ussize := abi.PaddedPieceSize(ssize).Unpadded()
	if sum != ussize {
		return nil, fmt.Errorf("aggregated piece sizes don't match sector size: %d != %d (%d)", sum, ussize, int64(ussize-sum))
	}

	// TODO: context cancellation respect
	p1o, err := ffi.SealPreCommitPhase1(
		sector.ProofType,
		paths.Cache,
		paths.Unsealed,
		paths.Sealed,
		sector.ID.Number,
		sector.ID.Miner,
		ticket,
		pieces,
	)
	if err != nil {
		return nil, fmt.Errorf("presealing sector %d (%s): %w", sector.ID.Number, paths.Unsealed, err)
	}
	p1odec := map[string]interface{}{}
	if err := json.Unmarshal(p1o, &p1odec); err != nil {
		return nil, fmt.Errorf("unmarshaling pc1 output: %w", err)
	}

	p1odec["_lotus_SealRandomness"] = ticket

	return json.Marshal(&p1odec)
}

var PC2CheckRounds = 3

func (sb *Sealer) SealPreCommit2(ctx context.Context, sector storage.SectorRef, phase1Out storage.PreCommit1Out) (storage.SectorCids, error) {
	paths, done, err := sb.sectors.AcquireSector(ctx, sector, storiface.FTSealed|storiface.FTCache, 0, storiface.PathSealing)
	if err != nil {
		return storage.SectorCids{}, fmt.Errorf("acquiring sector paths: %w", err)
	}
	defer done()

	sealedCID, unsealedCID, err := ffi.SealPreCommitPhase2(phase1Out, paths.Cache, paths.Sealed)
	if err != nil {
		return storage.SectorCids{}, fmt.Errorf("presealing sector %d (%s): %w", sector.ID.Number, paths.Unsealed, err)
	}

	ssize, err := sector.ProofType.SectorSize()
	if err != nil {
		return storage.SectorCids{}, fmt.Errorf("get ssize: %w", err)
	}

	p1odec := map[string]interface{}{}
	if err := json.Unmarshal(phase1Out, &p1odec); err != nil {
		return storage.SectorCids{}, fmt.Errorf("unmarshaling pc1 output: %w", err)
	}

	var ticket abi.SealRandomness
	ti, found := p1odec["_lotus_SealRandomness"]

	if found {
		ticket, err = base64.StdEncoding.DecodeString(ti.(string))
		if err != nil {
			return storage.SectorCids{}, fmt.Errorf("decoding ticket: %w", err)
		}

		for i := 0; i < PC2CheckRounds; i++ {
			var sd [32]byte
			_, _ = rand.Read(sd[:])

			_, err := ffi.SealCommitPhase1(
				sector.ProofType,
				sealedCID,
				unsealedCID,
				paths.Cache,
				paths.Sealed,
				sector.ID.Number,
				sector.ID.Miner,
				ticket,
				sd[:],
				[]abi.PieceInfo{{Size: abi.PaddedPieceSize(ssize), PieceCID: unsealedCID}},
			)
			if err != nil {
				log.Warn("checking PreCommit failed: ", err)
				log.Warnf("num:%d tkt:%v seed:%v sealedCID:%v, unsealedCID:%v", sector.ID.Number, ticket, sd[:], sealedCID, unsealedCID)

				return storage.SectorCids{}, fmt.Errorf("checking PreCommit failed: %w", err)
			}
		}
	}

	return storage.SectorCids{
		Unsealed: unsealedCID,
		Sealed:   sealedCID,
	}, nil
}

func (sb *Sealer) FinalizeSector(ctx context.Context, sector storage.SectorRef, keepUnsealed []storage.Range) error {
	ssize, err := sector.ProofType.SectorSize()
	if err != nil {
		return err
	}
	maxPieceSize := abi.PaddedPieceSize(ssize)

	if len(keepUnsealed) > 0 {

		sr := pieceRun(0, maxPieceSize)

		for _, s := range keepUnsealed {
			si := &rlepluslazy.RunSliceIterator{}
			if s.Offset != 0 {
				si.Runs = append(si.Runs, rlepluslazy.Run{Val: false, Len: uint64(s.Offset)})
			}
			si.Runs = append(si.Runs, rlepluslazy.Run{Val: true, Len: uint64(s.Size)})

			var err error
			sr, err = rlepluslazy.Subtract(sr, si)
			if err != nil {
				return err
			}
		}

		paths, done, err := sb.sectors.AcquireSector(ctx, sector, storiface.FTUnsealed, 0, storiface.PathStorage)
		if err != nil {
			return fmt.Errorf("acquiring sector cache path: %w", err)
		}
		defer done()

		pf, err := openPartialFile(maxPieceSize, paths.Unsealed)
		if err == nil {
			var at uint64
			for sr.HasNext() {
				r, err := sr.NextRun()
				if err != nil {
					_ = pf.Close()
					return err
				}

				offset := at
				at += r.Len
				if !r.Val {
					continue
				}

				err = pf.Free(storiface.PaddedByteIndex(abi.UnpaddedPieceSize(offset).Padded()), abi.UnpaddedPieceSize(r.Len).Padded())
				if err != nil {
					_ = pf.Close()
					return fmt.Errorf("free partial file range: %w", err)
				}
			}

			if err := pf.Close(); err != nil {
				return err
			}
		} else {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("opening partial file: %w", err)
			}
		}

	}

	paths, done, err := sb.sectors.AcquireSector(ctx, sector, storiface.FTCache, 0, storiface.PathStorage)
	if err != nil {
		return fmt.Errorf("acquiring sector cache path: %w", err)
	}
	defer done()

	return ffi.ClearCache(uint64(ssize), paths.Cache)
}

func GetRequiredPadding(oldLength abi.PaddedPieceSize, newPieceLength abi.PaddedPieceSize) ([]abi.PaddedPieceSize, abi.PaddedPieceSize) {

	padPieces := make([]abi.PaddedPieceSize, 0)

	toFill := uint64(-oldLength % newPieceLength)

	n := bits.OnesCount64(toFill)
	var sum abi.PaddedPieceSize
	for i := 0; i < n; i++ {
		next := bits.TrailingZeros64(toFill)
		psize := uint64(1) << uint(next)
		toFill ^= psize

		padded := abi.PaddedPieceSize(psize)
		padPieces = append(padPieces, padded)
		sum += padded
	}

	return padPieces, sum
}

func GenerateUnsealedCID(proofType abi.RegisteredSealProof, pieces []abi.PieceInfo) (cid.Cid, error) {
	ssize, err := proofType.SectorSize()
	if err != nil {
		return cid.Undef, err
	}

	pssize := abi.PaddedPieceSize(ssize)
	allPieces := make([]abi.PieceInfo, 0, len(pieces))
	if len(pieces) == 0 {
		allPieces = append(allPieces, abi.PieceInfo{
			Size:     pssize,
			PieceCID: zerocomm.ZeroPieceCommitment(pssize.Unpadded()),
		})
	} else {
		var sum abi.PaddedPieceSize

		padTo := func(pads []abi.PaddedPieceSize) {
			for _, p := range pads {
				allPieces = append(allPieces, abi.PieceInfo{
					Size:     p,
					PieceCID: zerocomm.ZeroPieceCommitment(p.Unpadded()),
				})

				sum += p
			}
		}

		for _, p := range pieces {
			ps, _ := GetRequiredPadding(sum, p.Size)
			padTo(ps)

			allPieces = append(allPieces, p)
			sum += p.Size
		}

		ps, _ := GetRequiredPadding(sum, pssize)
		padTo(ps)
	}

	return ffi.GenerateUnsealedCID(proofType, allPieces)
}
