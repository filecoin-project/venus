package chain

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/pkg/errors"
)

// IsReorg determines if choosing the end of the newChain as the new head
// would cause a "reorg" given the current head is at curHead.
// A reorg occurs when the old head is not a member of the new chain AND the
// old head is not a subset of the new head.
func IsReorg(old, new, commonAncestor *block.TipSet) bool {
	oldSortedSet := old.Key()
	newSortedSet := new.Key()

	return !(&newSortedSet).ContainsAll(oldSortedSet) && !commonAncestor.Equals(old)
}

// ReorgDiff returns the dropped and added block heights resulting from the
// reorg given the old and new heads and their common ancestor.
func ReorgDiff(old, new, commonAncestor *block.TipSet) (abi.ChainEpoch, abi.ChainEpoch, error) {
	hOld, err := old.Height()
	if err != nil {
		return 0, 0, err
	}

	hNew, err := new.Height()
	if err != nil {
		return 0, 0, err
	}

	hCommon, err := commonAncestor.Height()
	if err != nil {
		return 0, 0, err
	}

	if hCommon > hOld || hCommon > hNew {
		return 0, 0, errors.New("invalid common ancestor")
	}

	return hOld - hCommon, hNew - hCommon, nil
}
