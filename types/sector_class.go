package types

// SectorClass represents the miner's chosen sector size and PoSt/seal proof
// partitions.
type SectorClass struct {
	poStProofPartitions  PoStProofPartitions
	poRepProofPartitions PoRepProofPartitions
	sectorSize           SectorSize
}

// NewTestSectorClass returns a SectorClass suitable for testing.
func NewTestSectorClass() SectorClass {
	return SectorClass{
		poRepProofPartitions: TwoPoRepProofPartitions,
		poStProofPartitions:  OnePoStProofPartition,
		sectorSize:           OneKiBSectorSize,
	}
}

// NewLiveSectorClass returns a SectorClass suitable for normal operation of a
// go-filecoin node.
func NewLiveSectorClass() SectorClass {
	return SectorClass{
		poRepProofPartitions: TwoPoRepProofPartitions,
		poStProofPartitions:  OnePoStProofPartition,
		sectorSize:           TwoHundredFiftySixMiBSectorSize,
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
func (sc *SectorClass) SectorSize() SectorSize {
	return sc.sectorSize
}
