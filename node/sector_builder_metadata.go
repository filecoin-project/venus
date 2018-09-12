package node

import (
	cbor "gx/ipfs/QmV6BQ6fFCf9eFHDuRxvguvqfKLZtZrxthgZvDfRCs4tMN/go-ipld-cbor"
	ds "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/repo"
)

func init() {
	cbor.RegisterCborType(SectorMetadata{})
	cbor.RegisterCborType(SealedSectorMetadata{})
	cbor.RegisterCborType(SectorBuilderMetadata{})
}

// SectorMetadata represent the persistent metadata associated with a UnsealedSector.
type SectorMetadata struct {
	NumBytesFree         uint64
	NumBytesUsed         uint64
	Pieces               []*PieceInfo
	UnsealedSectorAccess string
}

// SealedSectorMetadata represent the persistent metadata associated with a SealedSector.
type SealedSectorMetadata struct {
	CommD                [32]byte
	CommR                [32]byte
	NumBytes             uint64
	Pieces               []*PieceInfo
	Proof                [192]byte
	SealedSectorAccess   string
	UnsealedSectorAccess string
}

// SectorBuilderMetadata represent the persistent metadata associated with a SectorBuilder.
type SectorBuilderMetadata struct {
	CurUnsealedSectorAccess        string
	MinerAddr                      address.Address
	SealedSectorReplicaCommitments [][32]byte
}

// SectorMetadata returns the metadata associated with a UnsealedSector.
func (s *UnsealedSector) SectorMetadata() *SectorMetadata {
	meta := &SectorMetadata{
		NumBytesFree:         s.numBytesFree,
		NumBytesUsed:         s.numBytesUsed,
		Pieces:               s.pieces,
		UnsealedSectorAccess: s.unsealedSectorAccess,
	}

	return meta
}

// SealedSectorMetadata returns the metadata associated with a SealedSector.
func (ss *SealedSector) SealedSectorMetadata() *SealedSectorMetadata {
	meta := &SealedSectorMetadata{
		CommD:                ss.commD,
		CommR:                ss.commR,
		NumBytes:             ss.numBytes,
		Pieces:               ss.pieces,
		SealedSectorAccess:   ss.sealedSectorAccess,
		Proof:                ss.proof,
		UnsealedSectorAccess: ss.unsealedSectorAccess,
	}

	return meta
}

// SectorBuilderMetadata returns the metadata associated with a SectorBuilderMetadata.
func (sb *SectorBuilder) SectorBuilderMetadata() *SectorBuilderMetadata {
	sb.sealedSectorsLk.RLock()
	defer sb.sealedSectorsLk.RUnlock()
	sb.curUnsealedSectorLk.RLock()
	defer sb.curUnsealedSectorLk.RUnlock()

	meta := SectorBuilderMetadata{
		MinerAddr:                      sb.MinerAddr,
		CurUnsealedSectorAccess:        sb.curUnsealedSector.unsealedSectorAccess,
		SealedSectorReplicaCommitments: make([][32]byte, len(sb.sealedSectors)),
	}
	for i, sealed := range sb.sealedSectors {
		meta.SealedSectorReplicaCommitments[i] = sealed.commR
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

type sectorMetadataStore struct {
	store repo.Datastore
}

// getSealedSector returns the sealed sector with the given replica commitment or an error if no match was found.
func (st *sectorMetadataStore) getSealedSector(commR [32]byte) (*SealedSector, error) {
	metadata, err := st.getSealedSectorMetadata(commR)
	if err != nil {
		return nil, err
	}

	return &SealedSector{
		commD:                metadata.CommD,
		commR:                metadata.CommR,
		numBytes:             metadata.NumBytes,
		pieces:               metadata.Pieces,
		sealedSectorAccess:   metadata.SealedSectorAccess,
		proof:                metadata.Proof,
		unsealedSectorAccess: metadata.UnsealedSectorAccess,
	}, nil
}

// getSector returns the sector with the given label or an error if no match was found.
func (st *sectorMetadataStore) getSector(label string) (*UnsealedSector, error) {
	metadata, err := st.getSectorMetadata(label)
	if err != nil {
		return nil, err
	}

	s := &UnsealedSector{
		numBytesFree:         metadata.NumBytesFree,
		numBytesUsed:         metadata.NumBytesUsed,
		pieces:               metadata.Pieces,
		unsealedSectorAccess: metadata.UnsealedSectorAccess,
	}

	return s, nil
}

// getSectorMetadata returns the metadata for a sector with the given label or an error if no match was found.
func (st *sectorMetadataStore) getSectorMetadata(label string) (*SectorMetadata, error) {
	key := metadataKey(label)

	data, err := st.store.Get(key)
	if err != nil {
		return nil, err
	}
	var m SectorMetadata
	if err := cbor.DecodeInto(data, &m); err != nil {
		return nil, err
	}
	return &m, err
}

// getSealedSectorMetadata returns the metadata for a sealed sector with the given replica commitment.
func (st *sectorMetadataStore) getSealedSectorMetadata(commR [32]byte) (*SealedSectorMetadata, error) {
	key := sealedMetadataKey(commR)

	data, err := st.store.Get(key)
	if err != nil {
		return nil, err
	}
	var m SealedSectorMetadata
	if err := cbor.DecodeInto(data, &m); err != nil {
		return nil, err
	}

	return &m, err
}

// getSectorBuilderMetadata returns the metadata for a miner's SectorBuilder.
func (st *sectorMetadataStore) getSectorBuilderMetadata(minerAddr address.Address) (*SectorBuilderMetadata, error) {
	key := builderMetadataKey(minerAddr)

	data, err := st.store.Get(key)
	if err != nil {
		return nil, err
	}
	var m SectorBuilderMetadata
	if err := cbor.DecodeInto(data, &m); err != nil {
		return nil, err
	}
	return &m, err
}

func (st *sectorMetadataStore) setSectorMetadata(label string, meta *SectorMetadata) error {
	key := metadataKey(label)
	data, err := cbor.DumpObject(meta)
	if err != nil {
		return err
	}
	return st.store.Put(key, data)
}

func (st *sectorMetadataStore) deleteSectorMetadata(label string) error {
	key := metadataKey(label)
	return st.store.Delete(key)
}

func (st *sectorMetadataStore) setSealedSectorMetadata(commR [32]byte, meta *SealedSectorMetadata) error {
	key := sealedMetadataKey(commR)
	data, err := cbor.DumpObject(meta)
	if err != nil {
		return err
	}
	return st.store.Put(key, data)
}

func (st *sectorMetadataStore) setSectorBuilderMetadata(minerAddress address.Address, meta *SectorBuilderMetadata) error {
	key := builderMetadataKey(minerAddress)
	data, err := cbor.DumpObject(meta)
	if err != nil {
		return err
	}
	return st.store.Put(key, data)
}

// TODO: Sealed sector metadata and sector metadata shouldn't exist in the
// datastore at the same time, and sector builder metadata needs to be kept
// in sync with sealed sector metadata (e.g. which sectors are sealed).
// This is the method to enforce these rules. Unfortunately this means that
// we're making more writes to the datastore than we really need to be
// doing. As the SectorBuilder evolves, we will introduce some checks which
// will optimize away redundant writes to the datastore.
func (sb *SectorBuilder) checkpoint(s *UnsealedSector) error {
	if err := sb.metadataStore.setSectorBuilderMetadata(sb.MinerAddr, sb.SectorBuilderMetadata()); err != nil {
		return errors.Wrap(err, "failed to save builder metadata")
	}

	if s.sealed == nil {
		if err := sb.metadataStore.setSectorMetadata(s.unsealedSectorAccess, s.SectorMetadata()); err != nil {
			return errors.Wrap(err, "failed to save sector metadata")
		}
	} else {
		if err := sb.metadataStore.setSealedSectorMetadata(s.sealed.commR, s.sealed.SealedSectorMetadata()); err != nil {
			return errors.Wrap(err, "failed to save sealed sector metadata")
		}

		if err := sb.metadataStore.deleteSectorMetadata(s.unsealedSectorAccess); err != nil {
			return errors.Wrap(err, "failed to remove sector metadata")
		}
	}

	return nil
}
