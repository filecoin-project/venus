package node

import (
	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
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
	MerkleRoot  []byte
}

// SealedSectorMetadata represent the persistent metadata associated with a SealedSector.
type SealedSectorMetadata struct {
	MerkleRoot      []byte
	Label           string
	BaseSectorLabel string
	SealedPath      string
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
		MerkleRoot:  []byte{},
	}
	if s.sealed != nil {
		meta.MerkleRoot = s.sealed.merkleRoot
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

func (sb *SectorBuilder) checkpoint() error {
	sector := sb.CurSector
	if err := sb.checkpointBuilderMeta(); err != nil {
		return err
	}
	if err := sb.checkpointSectorMeta(sector); err != nil {
		return err
	}
	if sector.sealed != nil {
		return sb.checkpointSealedMeta(sector.sealed)
	}
	return nil
}

// TODO: only actually set if changed.
func (sb *SectorBuilder) checkpointSectorMeta(s *Sector) error {
	return sb.setMeta(s.Label, s.SectorMetadata())
}

func (sb *SectorBuilder) checkpointSealedMeta(ss *SealedSector) error {
	return sb.setSealedMeta(ss.merkleRoot, ss.SealedSectorMetadata())
}

func (sb *SectorBuilder) checkpointBuilderMeta() error {
	return sb.setBuilderMeta(sb.MinerAddr, sb.SectorBuilderMetadata())
}
