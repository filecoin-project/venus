package types

// SectorSize is the amount of bytes in a sector. This amount will be slightly
// greater than the number of user bytes which can be written to a sector due to
// bit-padding.
type SectorSize uint64

const (
	OneKiBSectorSize = SectorSize(iota)
	TwoHundredFiftySixMiBSectorSize
)
