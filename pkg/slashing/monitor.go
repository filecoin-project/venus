package slashing

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/types"
	"sync"
)

// ConsensusFaultDetector detects consensus faults -- misbehavior conditions where a single
// party produces multiple blocks at the same time.
type ConsensusFaultDetector struct {
	// minerIndex tracks witnessed blocks by miner address and epoch
	minerIndex map[address.Address]map[abi.ChainEpoch]*types.BlockHeader
	// sender sends messages on behalf of the slasher
	faultCh chan ConsensusFault
	lk      sync.Mutex
}

// ConsensusFault is the information needed to submit a consensus fault
type ConsensusFault struct {
	// Block1 and Block2 are two distinct blocks from an overlapping interval
	// signed by the same miner
	Block1, Block2 *types.BlockHeader
}

// NewConsensusFaultDetector returns a fault detector given a fault channel
func NewConsensusFaultDetector(faultCh chan ConsensusFault) *ConsensusFaultDetector {
	return &ConsensusFaultDetector{
		minerIndex: make(map[address.Address]map[abi.ChainEpoch]*types.BlockHeader),
		faultCh:    faultCh,
	}

}

// CheckBlock records a new block and checks for faults
// Preconditions: the signature is already checked and p is the parent
func (detector *ConsensusFaultDetector) CheckBlock(b *types.BlockHeader, p *types.TipSet) error {
	latest := b.Height
	parentHeight := p.Height()
	earliest := parentHeight + 1

	// Find per-miner index
	detector.lk.Lock()
	defer detector.lk.Unlock()
	blockByEpoch, tracked := detector.minerIndex[b.Miner]
	if !tracked {
		blockByEpoch = make(map[abi.ChainEpoch]*types.BlockHeader)
		detector.minerIndex[b.Miner] = blockByEpoch
	}

	// Add this epoch to the miner's index, emitting any detected faults
	for e := earliest; e <= latest; e++ {
		collision, tracked := blockByEpoch[e]
		if tracked {
			// Exact duplicates are not faults
			if collision.Cid().Equals(b.Cid()) {
				continue
			}
			// Emit all faults, any special handling of duplicates belongs downstream
			detector.faultCh <- ConsensusFault{b, collision}
		}
		// In case of collision overwrite with most recent
		blockByEpoch[e] = b
	}
	return nil
}
