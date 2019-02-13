package chain

import (
	"github.com/filecoin-project/go-filecoin/types"
)

// IsReorg determines if choosing the end of the newChain as the new head
// would cause a "reorg" given the current head is at curHead.
// A reorg occurs when curHead is not a member of newChain AND curHead is not
// a subset of newChain's head.
func IsReorg(curHead types.TipSet, newChain []types.TipSet) bool {
	newHead := newChain[len(newChain)-1]
	oldSortedSet := curHead.ToSortedCidSet()
	newSortedSet := newHead.ToSortedCidSet()
	if (&newSortedSet).Contains(&oldSortedSet) {
		return false
	}
	for _, ancestor := range newChain {
		if ancestor.Equals(curHead) {
			return false
		}
	}
	return true
}
