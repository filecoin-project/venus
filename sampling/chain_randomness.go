package sampling

import (
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

// SampleChainRandomness produces a slice of bytes (a ticket) sampled from the highest tipset with
// height less than or equal to `sampleHeight`.
// The tipset slice must be sorted by descending block height.
func SampleChainRandomness(sampleHeight *types.BlockHeight, tipSetsDescending []types.TipSet) ([]byte, error) {
	if sampleHeight.LessThan(types.NewBlockHeight(0)) {
		return nil, errors.Errorf("can't sample chain at negative height %s", sampleHeight)
	}
	if len(tipSetsDescending) == 0 {
		return nil, errors.New("can't sample empty chain segment")
	}
	// Find the first (highest) tipset with height less than or equal to sampleHeight.
	// This is more complex than necessary: https://github.com/filecoin-project/go-filecoin/issues/3025
	sampleIndex := -1
	for i, tip := range tipSetsDescending {
		height, err := tip.Height()
		if err != nil {
			return nil, errors.Wrapf(err, "failed sampling chain segment")
		}

		if types.NewBlockHeight(height).LessEqual(sampleHeight) {
			sampleIndex = i
			break
		}
	}

	// Produce an error if the slice does not include any tipsets at least as low as `sampleHeight`.
	if sampleIndex == -1 {
		lastIdx := len(tipSetsDescending) - 1
		lowestAvailableHeight, err := tipSetsDescending[lastIdx].Height()
		if err != nil {
			return nil, errors.Wrap(err, "failed to read chain segment height")
		}

		return nil, errors.Errorf("sample height %s out of range %d...", sampleHeight, lowestAvailableHeight)
	}

	ticket, err := tipSetsDescending[sampleIndex].MinTicket()
	if err != nil {
		return nil, err
	}
	return ticket.SortKey(), nil
}
