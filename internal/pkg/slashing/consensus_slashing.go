package slashing

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// CFaultDetector detects consensus faults -- misbehavior conditions where a single
// party produces multiple blocks at the same time.
type CFaultDetector struct {
	// minerIndex tracks witnessed blocks by miner address and epoch
	minerIndex map[address.Address]map[uint64]*block.Block
	// sender sends messages on behalf of the slasher
	faultCh chan ConsensusFault
}

// ConsensusFault is the information needed to submit a consensus fault
type ConsensusFault struct {
	Block1, Block2 *block.Block
}

// NewCFaultDetector returns a fault detector given a fault channel
func NewCFaultDetector(faultCh chan ConsensusFault) *CFaultDetector {
	return &CFaultDetector{
		minerIndex: make(map[address.Address]map[uint64]*block.Block),
		faultCh:    faultCh,
	}

}

// CheckBlock records a new block and checks for faults
// Preconditions: the signature is already checked and p is the parent
func (detector *CFaultDetector) CheckBlock(b *block.Block, p block.TipSet) error {
	latest := uint64(b.Height)
	parentHeight, err := p.Height()
	if err != nil {
		return err
	}
	earliest := parentHeight + 1

	// Find per-miner index
	blockByEpoch, tracked := detector.minerIndex[b.Miner]
	if !tracked {
		blockByEpoch = make(map[uint64]*block.Block)
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
