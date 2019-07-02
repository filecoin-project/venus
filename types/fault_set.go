package types

import (
	cbor "github.com/ipfs/go-ipld-cbor"
)

func init() {
	cbor.RegisterCborType(FaultSet{})
}

// FaultSet indicates sectors that have failed during a proving period
//
// https://github.com/filecoin-project/specs/blob/master/data-structures.md#faultset
type FaultSet struct {
	// Offset is the offset from the start of the proving period
	Offset uint64 `json:"offset"`

	// SectorIds are the faulted sectors
	SectorIds IntSet `json:"sectorIds"`
}
