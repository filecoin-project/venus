package sampling

import (
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

// LookbackParameter defines how many non-empty tiptsets (not rounds) earlier than any sample
// height (in rounds) from which to sample the chain for randomness.
// This constant is a protocol (actor) parameter and should be defined in actor code.
const LookbackParameter = 3

// SampleChainRandomness produces a slice of bytes (a ticket) sampled from the tipset `LookbackParameter`
// tipsets (not rounds) prior to the highest tipset with height less than or equal to `sampleHeight`.
// The tipset slice must be sorted by descending block height.
func SampleChainRandomness(sampleHeight *types.BlockHeight, tipSetsDescending []types.TipSet) ([]byte, error) {
	// Find the first (highest) tipset with height less than or equal to sampleHeight.
	// This is more complex than necessary: https://github.com/filecoin-project/go-filecoin/issues/3025
	sampleIndex := -1
	for i, tip := range tipSetsDescending {
		height, err := tip.Height()
		if err != nil {
			return nil, errors.Wrap(err, "error obtaining tip set height")
		}

		if types.NewBlockHeight(height).LessEqual(sampleHeight) {
			sampleIndex = i
			break
		}
	}

	// Produce an error if the slice does not include any tipsets at least as low as `sampleHeight`.
	if sampleIndex == -1 {
		return nil, errors.Errorf("sample height out of range: %s", sampleHeight)
	}

	// Now look LookbackParameter tipsets (not rounds) prior to the sample tipset.
	lookbackIdx := sampleIndex + LookbackParameter
	lastIdx := len(tipSetsDescending) - 1
	if lookbackIdx > lastIdx {
		// If this index farther than the lowest height (last) tipset in the slice, then
		// - if the lowest is the genesis, use that, else
		// - error (the tipset slice is insufficient)
		//
		// TODO: security, spec, bootstrap implications.
		// See issue https://github.com/filecoin-project/go-filecoin/issues/1872
		lowestAvailableHeight, err := tipSetsDescending[lastIdx].Height()
		if err != nil {
			return nil, errors.Wrap(err, "error obtaining tip set height")
		}

		if lowestAvailableHeight == uint64(0) {
			lookbackIdx = lastIdx
		} else {
			errMsg := "sample height out of range: lookbackIdx=%d, lastHeightInChain=%d"
			return nil, errors.Errorf(errMsg, lookbackIdx, lowestAvailableHeight)
		}
	}

	return tipSetsDescending[lookbackIdx].MinTicket()
}
