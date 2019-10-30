package miner

import (
	"strconv"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
)

// Note that this type's interface is a bit of a placeholder.  We expect to
// move to something like actor.Lookup eventually which will manage its own
// hash linked storage.  See #2887.  This will require some interface changes.

// SectorSet is a collection of sector commitments indexed by their integer
// sectorID.  Due to a bug in refmt, the sector id-keys need to be stringified.
// See also: https://github.com/polydawn/refmt/issues/35.
type SectorSet map[string]types.Commitments

// NewSectorSet initializes a SectorSet with no entries.
func NewSectorSet() SectorSet {
	return make(map[string]types.Commitments)
}

// Has returns true if the SectorSet is already tracking id, else false.
func (ss SectorSet) Has(id uint64) bool {
	_, ok := ss[idStr(id)]
	return ok
}

// Add updates the SectorSet to include the new commitment at the given id.
func (ss SectorSet) Add(id uint64, comms types.Commitments) {
	ss[idStr(id)] = comms
}

// Get returns the commitment at the given id and a bool indicating success.
func (ss SectorSet) Get(id uint64) (types.Commitments, bool) {
	comms, ok := ss[idStr(id)]
	return comms, ok
}

// Drop removes all the provided sectorIDs from the collection, erroring if any
// do not exist in the SectorSet.
func (ss SectorSet) Drop(ids []uint64) error {
	for _, id := range ids {
		if _, ok := ss[idStr(id)]; !ok {
			return Errors[ErrInvalidSector]
		}
		delete(ss, idStr(id))
	}
	return nil
}

// IDs returns all of the sectorIDs in the sectorset.  They are not sorted.
func (ss SectorSet) IDs() ([]uint64, error) {
	var ids []uint64
	for idStr := range ss {
		id, err := str2ID(idStr)
		if err != nil {
			return nil, errors.RevertErrorWrap(err, "corrupt sectorset id")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// Size returns the number of sectorIDs in the SectorSet.
func (ss SectorSet) Size() int {
	return len(ss)
}

// TODO: use uint64 as map keys instead of this abomination, once refmt is fixed.
// https://github.com/polydawn/refmt/issues/35
func idStr(sectorID uint64) string {
	return strconv.FormatUint(sectorID, 10)
}

func str2ID(idStr string) (uint64, error) {
	return strconv.ParseUint(idStr, 10, 64)
}
