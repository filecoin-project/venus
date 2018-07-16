package prettyprint

import (
	"github.com/filecoin-project/go-filecoin/types"
)

// StringFromBlocks returns a string representation of the input block CIDs
// formatted for printing.
func StringFromBlocks(blks []*types.Block) string {
	s := types.SortedCidSet{}
	for _, b := range blks {
		s.Add(b.Cid())
	}
	return s.String()
}
