package types

// SectorSize is the amount of bytes in a sector. This amount will be slightly
// greater than the number of user bytes which can be written to a sector due to
// bit-padding.
type SectorSize int

const (
	UnknownSectorSize = SectorSize(iota)
	OneKiBSectorSize
	TwoHundredFiftySixMiBSectorSize
)

func (s SectorSize) Uint64() uint64 {
	switch s {
	case OneKiBSectorSize:
		return 1024
	case TwoHundredFiftySixMiBSectorSize:
		return 266338304
	default:
		return 0
	}
}
