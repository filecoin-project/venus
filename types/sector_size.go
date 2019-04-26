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

// Uint64 produces the number of bytes which the miner will get power for when
// committing a sector of this size to the network.
func (s SectorSize) Uint64() uint64 {
	switch s {
	case OneKiBSectorSize:
		return 1024
	case TwoHundredFiftySixMiBSectorSize:
		return 1 << 28
	default:
		return 0
	}
}
