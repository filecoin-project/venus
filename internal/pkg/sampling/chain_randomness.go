package sampling

import (
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
)

// SampleNthTicket produces a ticket sampled from the nth tipset in the
// provided ancestor slice.  It handles sampling from genesis.
func SampleNthTicket(n int, tipSetsDescending []block.TipSet) (block.Ticket, error) {
	if len(tipSetsDescending) == 0 {
		return block.Ticket{}, errors.New("can't sample empty chain segment")
	}
	lastIdx := len(tipSetsDescending) - 1
	if n > lastIdx {
		// Handle chain startup
		lowestAvailableHeight, err := tipSetsDescending[lastIdx].Height()
		if err != nil {
			return block.Ticket{}, errors.Wrap(err, "failed to read chain segment height")
		}
		// Return genesis ticket if this is the edge case where
		// tipSetsDescending run all the way back to genesis
		if lowestAvailableHeight == 0 {
			return tipSetsDescending[lastIdx].MinTicket()
		}
		return block.Ticket{}, errors.Errorf("can't sample ticket %d from %d tipsets", n, lastIdx+1)
	}

	return tipSetsDescending[n].MinTicket()
}
