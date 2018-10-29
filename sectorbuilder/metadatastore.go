package sectorbuilder

import (
	cbor "gx/ipfs/QmV6BQ6fFCf9eFHDuRxvguvqfKLZtZrxthgZvDfRCs4tMN/go-ipld-cbor"
	ds "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/repo"
)

func init() {
	cbor.RegisterCborType(unsealedSectorMetadata{})
	cbor.RegisterCborType(sealedSectorMetadata{})
	cbor.RegisterCborType(metadata{})
}

// unsealedSectorMetadata represents the persistent metadata associated with a UnsealedSector.
type unsealedSectorMetadata struct {
	MaxBytes             uint64
	NumBytesUsed         uint64
	Pieces               []*PieceInfo
	SectorID             uint64
	UnsealedSectorAccess string
}

// sealedSectorMetadata represent the persistent metadata associated with a SealedSector.
type sealedSectorMetadata struct {
	CommD                [32]byte
	CommR                [32]byte
	NumBytes             uint64
	Pieces               []*PieceInfo
	Proof                [384]byte
	SealedSectorAccess   string
	SectorID             uint64
	UnsealedSectorAccess string
}

// metadata represent the persistent metadata associated with a SectorBuilder.
type metadata struct {
	CurUnsealedSectorAccess string
	MinerAddr               address.Address
	SealedSectorCommitments [][32]byte
}

// dumpCurrentState returns the metadata associated with a UnsealedSector.
func (s *UnsealedSector) dumpCurrentState() *unsealedSectorMetadata {
	meta := &unsealedSectorMetadata{
		MaxBytes:             s.maxBytes,
		NumBytesUsed:         s.numBytesUsed,
		Pieces:               s.pieces,
		SectorID:             s.SectorID,
		UnsealedSectorAccess: s.unsealedSectorAccess,
	}

	return meta
}

// dumpCurrentState returns the metadata associated with a SealedSector.
func (ss *SealedSector) dumpCurrentState() *sealedSectorMetadata {
	meta := &sealedSectorMetadata{
		CommD:                ss.CommD,
		CommR:                ss.CommR,
		NumBytes:             ss.numBytes,
		Pieces:               ss.pieces,
		Proof:                ss.proof,
		SealedSectorAccess:   ss.sealedSectorAccess,
		SectorID:             ss.SectorID,
		UnsealedSectorAccess: ss.unsealedSectorAccess,
	}

	return meta
}

// dumpCurrentState returns the current state of the sector builder.
func (sb *defaultSectorBuilder) dumpCurrentState() *metadata {
	meta := metadata{
		CurUnsealedSectorAccess: sb.curUnsealedSector.unsealedSectorAccess,
		MinerAddr:               sb.minerAddr,
		SealedSectorCommitments: make([][32]byte, len(sb.sealedSectors)),
	}
	for i, sealed := range sb.sealedSectors {
		meta.SealedSectorCommitments[i] = sealed.CommR
	}
	return &meta
}

func metadataKey(label string) ds.Key {
	path := []string{"sectors", "metadata"}
	return ds.KeyWithNamespaces(path).Instance(label)
}

func sealedMetadataKey(commR [32]byte) ds.Key {
	path := []string{"sealedSectors", "metadata"}
	return ds.KeyWithNamespaces(path).Instance(commRString(commR))
}

func builderMetadataKey(minerAddress address.Address) ds.Key {
	path := []string{"builder", "metadata"}
	return ds.KeyWithNamespaces(path).Instance(minerAddress.String())
}

type metadataStore struct {
	store repo.Datastore
}

// unsealArgs is a struct holding the arguments to pass to the Prover#Unseal method for a given piece.
type unsealArgs struct {
	sealedSectorAccess string
	startOffset        uint64
	numBytes           uint64
	sectorID           uint64
}

func (st *metadataStore) getUnsealArgsForPiece(minerAddr address.Address, pieceCid *cid.Cid) (unsealArgs, error) {
	metadata, err := st.getSectorBuilderMetadata(minerAddr)
	if err != nil {
		return unsealArgs{}, errors.Wrapf(err, "failed to get sector builder metadata for miner with addr %s", minerAddr.String())
	}

	for _, commR := range metadata.SealedSectorCommitments {
		sealedSector, err := st.getSealedSector(commR)
		if err != nil {
			return unsealArgs{}, errors.Wrapf(err, "failed to get sealed sector with commR %s", pieceCid.String())
		}

		offset := uint64(0)
		for _, pieceInfo := range sealedSector.pieces {
			if pieceInfo.Ref.Equals(pieceCid) {
				return unsealArgs{
					sectorID:           sealedSector.SectorID,
					startOffset:        offset,
					numBytes:           pieceInfo.Size,
					sealedSectorAccess: sealedSector.sealedSectorAccess,
				}, nil
			}
			offset += pieceInfo.Size
		}
	}

	return unsealArgs{}, errors.Errorf("failed to find a sealed sector holding piece with cid %s", pieceCid.String())
}

// getSealedSector returns the sealed sector with the given replica commitment or an error if no match was found.
func (st *metadataStore) getSealedSector(commR [32]byte) (*SealedSector, error) {
	metadata, err := st.getSealedSectorMetadata(commR)
	if err != nil {
		return nil, err
	}

	return &SealedSector{
		CommD:                metadata.CommD,
		CommR:                metadata.CommR,
		numBytes:             metadata.NumBytes,
		pieces:               metadata.Pieces,
		proof:                metadata.Proof,
		sealedSectorAccess:   metadata.SealedSectorAccess,
		SectorID:             metadata.SectorID,
		unsealedSectorAccess: metadata.UnsealedSectorAccess,
	}, nil
}

// getSector returns the sector with the given label or an error if no match was found.
func (st *metadataStore) getSector(label string) (*UnsealedSector, error) {
	metadata, err := st.getSectorMetadata(label)
	if err != nil {
		return nil, err
	}

	s := &UnsealedSector{
		SectorID:             metadata.SectorID,
		maxBytes:             metadata.MaxBytes,
		numBytesUsed:         metadata.NumBytesUsed,
		pieces:               metadata.Pieces,
		unsealedSectorAccess: metadata.UnsealedSectorAccess,
	}

	return s, nil
}

// getSectorMetadata returns the metadata for a sector with the given label or an error if no match was found.
func (st *metadataStore) getSectorMetadata(label string) (*unsealedSectorMetadata, error) {
	key := metadataKey(label)

	data, err := st.store.Get(key)
	if err != nil {
		return nil, err
	}
	var m unsealedSectorMetadata
	if err := cbor.DecodeInto(data, &m); err != nil {
		return nil, err
	}
	return &m, err
}

// getSealedSectorMetadata returns the metadata for a sealed sector with the given replica commitment.
func (st *metadataStore) getSealedSectorMetadata(commR [32]byte) (*sealedSectorMetadata, error) {
	key := sealedMetadataKey(commR)

	data, err := st.store.Get(key)
	if err != nil {
		return nil, err
	}
	var m sealedSectorMetadata
	if err := cbor.DecodeInto(data, &m); err != nil {
		return nil, err
	}

	return &m, err
}

// getSectorBuilderMetadata returns the metadata for a miner's SectorBuilder.
func (st *metadataStore) getSectorBuilderMetadata(minerAddr address.Address) (*metadata, error) {
	key := builderMetadataKey(minerAddr)

	data, err := st.store.Get(key)
	if err != nil {
		return nil, err
	}
	var m metadata
	if err := cbor.DecodeInto(data, &m); err != nil {
		return nil, err
	}
	return &m, err
}

func (st *metadataStore) setSectorMetadata(label string, meta *unsealedSectorMetadata) error {
	key := metadataKey(label)
	data, err := cbor.DumpObject(meta)
	if err != nil {
		return err
	}
	return st.store.Put(key, data)
}

func (st *metadataStore) deleteSectorMetadata(label string) error {
	key := metadataKey(label)
	return st.store.Delete(key)
}

func (st *metadataStore) setSealedSectorMetadata(commR [32]byte, meta *sealedSectorMetadata) error {
	key := sealedMetadataKey(commR)
	data, err := cbor.DumpObject(meta)
	if err != nil {
		return err
	}
	return st.store.Put(key, data)
}

func (st *metadataStore) setSectorBuilderMetadata(minerAddress address.Address, meta *metadata) error {
	key := builderMetadataKey(minerAddress)
	data, err := cbor.DumpObject(meta)
	if err != nil {
		return err
	}
	return st.store.Put(key, data)
}
