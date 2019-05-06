package types

// SectorClass represents the miner's chosen sector size and PoSt/seal proof
// partitions.
type SectorClass struct {
	poStProofPartitions  PoStProofPartitions
	poRepProofPartitions PoRepProofPartitions
	sectorSize           *BytesAmount
}

// NewSectorClass returns a sector class configured with the provided sector
// size. Note that there must exist a published Groth parameter and verifying
// key in order for the sector size to be usable. A miner configured to use a
// sector size for which we have not published Groth parameters will fail to
// prove their storage.
func NewSectorClass(sectorSize *BytesAmount) SectorClass {
	return SectorClass{
		poRepProofPartitions: TwoPoRepProofPartitions,
		poStProofPartitions:  OnePoStProofPartition,
		sectorSize:           sectorSize,
	}
}

// PoRepProofPartitions returns the sector class's PoRep proof partitions
func (sc *SectorClass) PoRepProofPartitions() PoRepProofPartitions {
	return sc.poRepProofPartitions
}

// PoStProofPartitions returns the sector class's PoSt proof partitions
func (sc *SectorClass) PoStProofPartitions() PoStProofPartitions {
	return sc.poStProofPartitions
}

// SectorSize returns the size of this sector class's sectors after bit-padding
// has been added. Note that the amount of user bytes that will fit into a
// sector is less than SectorSize due to bit-padding.
func (sc *SectorClass) SectorSize() *BytesAmount {
	return sc.sectorSize
}
