package types

import (
	"fmt"
)

// FaultSet indicates sectors that have failed PoSt during a proving period
//
// https://github.com/filecoin-project/specs/blob/master/data-structures.md#faultset
type FaultSet struct {
	// Offset is the offset from the start of the proving period, currently unused
	Offset uint64 `json:"offset,omitempty" refmt:"idx"`

	// SectorIds are the faulted sectors
	SectorIds IntSet `json:"sectorIds" refmt:"ids"`
}

// NewFaultSet constructs a FaultSet from a slice of sector ids that have faulted
func NewFaultSet(sectorIds []uint64) FaultSet {
	return FaultSet{SectorIds: NewIntSet(sectorIds...)}
}

// EmptyFaultSet constructs an empty FaultSet as a convenience
func EmptyFaultSet() FaultSet {
	return FaultSet{SectorIds: EmptyIntSet()}
}

// String returns a printable representation of the FaultSet
func (fs FaultSet) String() string {
	return fmt.Sprintf("%d / %s", fs.Offset, fs.SectorIds.String())
}
