package consensus

import "github.com/filecoin-project/specs-actors/actors/abi"

// FinalityEpochs is the number of epochs between an accepted head and the
// first finalized tipset.
const FinalityEpochs = abi.ChainEpoch(500)
