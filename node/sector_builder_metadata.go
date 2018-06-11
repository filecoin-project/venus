package node

import (
	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"

	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(SectorMetadata{})
	cbor.RegisterCborType(SealedSectorMetadata{})
	cbor.RegisterCborType(SectorBuilderMetadata{})
}

// SectorMetadata represent the persistent metadata associated with a Sector.
type SectorMetadata struct {
	StagingPath string
	Pieces      []*PieceInfo
	Size        uint64
	Free        uint64
}

// SealedSectorMetadata represent the persistent metadata associated with a SealedSector.
type SealedSectorMetadata struct {
	MerkleRoot      []byte
	Label           string
	BaseSectorLabel string
	SealedPath      string
	Pieces          []*PieceInfo
	Size            uint64
}

// SectorBuilderMetadata represent the persistent metadata associated with a SectorBuilder.
type SectorBuilderMetadata struct {
	Miner         types.Address
	Sectors       []string
	SealedSectors []string
}

// SectorMetadata returns the metadata associated with a Sector.
func (s *Sector) SectorMetadata() *SectorMetadata {
	meta := &SectorMetadata{
		StagingPath: s.filename,
		Pieces:      s.Pieces,
		Size:        s.Size,
		Free:        s.Free,
	}

	return meta
}

// SealedSectorMetadata returns the metadata associated with a SealedSector.
func (ss *SealedSector) SealedSectorMetadata() *SealedSectorMetadata {
	meta := &SealedSectorMetadata{
		MerkleRoot:      ss.merkleRoot,
		Label:           ss.label,
		BaseSectorLabel: ss.baseSector.Label,
		SealedPath:      ss.filename,
		Size:            ss.baseSector.Size,
		Pieces:          ss.baseSector.Pieces,
	}
	return meta
}

// SectorBuilderMetadata returns the metadata associated with a SectorBuilderMetadata.
func (sb *SectorBuilder) SectorBuilderMetadata() *SectorBuilderMetadata {
	meta := SectorBuilderMetadata{
		Miner:         sb.MinerAddr,
		Sectors:       []string{sb.CurSector.Label},
		SealedSectors: make([]string, len(sb.SealedSectors)),
	}
	for i, sealed := range sb.SealedSectors {
		meta.SealedSectors[i] = sealed.label
	}
	return &meta
}

func (sb *SectorBuilder) metadataKey(label string) ds.Key {
	path := []string{"sectors", "metadata"}
	return ds.KeyWithNamespaces(path).Instance(label)
}

func (sb *SectorBuilder) sealedMetadataKey(merkleRoot []byte) ds.Key {
	path := []string{"sealedSectors", "metadata"}
	return ds.KeyWithNamespaces(path).Instance(merkleString(merkleRoot))
}

func (sb *SectorBuilder) builderMetadataKey(minerAddress types.Address) ds.Key {
	path := []string{"sectors", "metadata"}
	return ds.KeyWithNamespaces(path).Instance(minerAddress.String())
}

// GetMeta returns SectorMetadata for sector labeled, label, and any error.
func (sb *SectorBuilder) GetMeta(label string) (*SectorMetadata, error) {
	key := sb.metadataKey(label)

	data, err := sb.store.Get(key)
	if err != nil {
		return nil, err
	}
	var m SectorMetadata
	if err := cbor.DecodeInto(data.([]byte), &m); err != nil {
		return nil, err
	}
	return &m, err
}

// GetSealedMeta returns SealedSectorMetadata for merkleRoot, and any error.
func (sb *SectorBuilder) GetSealedMeta(merkleRoot []byte) (*SealedSectorMetadata, error) {
	key := sb.sealedMetadataKey(merkleRoot)

	data, err := sb.store.Get(key)
	if err != nil {
		return nil, err
	}
	var m SealedSectorMetadata
	if err := cbor.DecodeInto(data.([]byte), &m); err != nil {
		return nil, err
	}

	return &m, err
}

// GetBuilderMeta returns SectorBuilderMetadata for SectorBuilder, sb, and any error.
func (sb *SectorBuilder) GetBuilderMeta(minerAddress types.Address) (*SectorBuilderMetadata, error) {
	key := sb.builderMetadataKey(minerAddress)

	data, err := sb.store.Get(key)
	if err != nil {
		return nil, err
	}
	var m SectorBuilderMetadata
	if err := cbor.DecodeInto(data.([]byte), &m); err != nil {
		return nil, err
	}
	return &m, err
}

func (sb *SectorBuilder) setMeta(label string, meta *SectorMetadata) error {
	key := sb.metadataKey(label)
	data, err := cbor.DumpObject(meta)
	if err != nil {
		return err
	}
	return sb.store.Put(key, data)
}

func (sb *SectorBuilder) unsetMeta(label string) error {
	key := sb.metadataKey(label)
	return sb.store.Delete(key)
}

func (sb *SectorBuilder) setSealedMeta(merkleRoot []byte, meta *SealedSectorMetadata) error {
	key := sb.sealedMetadataKey(merkleRoot)
	data, err := cbor.DumpObject(meta)
	if err != nil {
		return err
	}
	return sb.store.Put(key, data)
}

func (sb *SectorBuilder) setBuilderMeta(minerAddress types.Address, meta *SectorBuilderMetadata) error {
	key := sb.metadataKey(minerAddress.String())
	data, err := cbor.DumpObject(meta)
	if err != nil {
		return err
	}
	return sb.store.Put(key, data)
}

// TODO: Sealed sector metadata and sector metadata shouldn't exist in the
// datastore at the same time, and sector builder metadata needs to be kept
// in sync with sealed sector metadata (e.g. which sectors are sealed).
// This is the method to enforce these rules. Unfortunately this means that
// we're making more writes to the datastore than we really need to be
// doing. As the SectorBuilder evolves, we will introduce some checks which
// will optimize away redundant writes to the datastore.
func (sb *SectorBuilder) checkpoint(s *Sector) error {
	if err := sb.setBuilderMeta(sb.MinerAddr, sb.SectorBuilderMetadata()); err != nil {
		return errors.Wrap(err, "failed to save builder metadata")
	}

	if s.sealed == nil {
		if err := sb.setMeta(s.Label, s.SectorMetadata()); err != nil {
			return errors.Wrap(err, "failed to save sector metadata")
		}
	} else {
		if err := sb.setSealedMeta(s.sealed.merkleRoot, s.sealed.SealedSectorMetadata()); err != nil {
			return errors.Wrap(err, "failed to save sealed sector metadata")
		}

		if err := sb.unsetMeta(s.Label); err != nil {
			return errors.Wrap(err, "failed to remove sector metadata")
		}
	}

	return nil
}
