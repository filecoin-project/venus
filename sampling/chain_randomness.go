package sampling

import (
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

// LookbackParameter is the protocol parameter defining how many blocks in the
// past to look back to sample randomness values.
const LookbackParameter = 3

// SampleChainRandomness produces a slice of random bytes sampled from a TipSet
// in the provided slice of TipSets at a given height (minus lookback). This
// function assumes that the tipSets slice is sorted and that the genesis block
// is at the end of the list.
//
// SampleChainRandomness is useful for things like PoSt challenge seed
// generation.
func SampleChainRandomness(sampleHeight *types.BlockHeight, tipSetsDescBlockHeight []types.TipSet) ([]byte, error) {
	sampleIndex := -1
	tipSetsLen := len(tipSetsDescBlockHeight)
	lastIdxInTipSets := tipSetsLen - 1

	for i := 0; i < tipSetsLen; i++ {
		height, err := tipSetsDescBlockHeight[i].Height()
		if err != nil {
			return nil, errors.Wrap(err, "error obtaining tip set height")
		}

		if types.NewBlockHeight(height).Equal(sampleHeight) {
			sampleIndex = i
			break
		}
	}

	// Produce an error if no tip set exists in `tipSetsDescBlockHeight` with
	// block height `sampleHeight`.
	if sampleIndex == -1 {
		return nil, errors.Errorf("sample height out of range: %s", sampleHeight)
	}

	// If looking backwards in time Lookback-number of tip sets from the tip set
	// with `sampleHeight` would put us farther back in time than the genesis
	// block, choose the last block in `tipSetsDescBlockHeight` for sampling. If
	// this block isn't the genesis block (because of some programmer error),
	// produce an error.
	//
	// TODO: security, spec, bootstrap implications.
	// See issue https://github.com/filecoin-project/go-filecoin/issues/1872
	lookbackIdx := sampleIndex + LookbackParameter
	if lookbackIdx > lastIdxInTipSets {
		leastHeightInChain, err := tipSetsDescBlockHeight[lastIdxInTipSets].Height()
		if err != nil {
			return nil, errors.Wrap(err, "error obtaining tip set height")
		}

		if leastHeightInChain == uint64(0) {
			lookbackIdx = lastIdxInTipSets
		} else {
			errMsg := "lookbackIdx=%d does not correspond to genesis block (leastHeightInChain=%d)"
			return nil, errors.Errorf(errMsg, lookbackIdx, leastHeightInChain)
		}
	}

	return tipSetsDescBlockHeight[lookbackIdx].MinTicket()
}
