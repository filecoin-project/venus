package gengen

import (
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v6/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/v6/actors/builtin/miner"
	tutil "github.com/filecoin-project/specs-actors/v6/support/testing"
	"github.com/filecoin-project/venus/pkg/constants"
)

// MakeCommitCfgs creates n gengen commit configs, casting strings to cids.
func MakeCommitCfgs(n int) ([]*CommitConfig, error) {
	cfgs := make([]*CommitConfig, n)
	for i := 0; i < n; i++ {
		commP := tutil.MakeCID(fmt.Sprintf("commP: %d", i), &market.PieceCIDPrefix)
		commR := tutil.MakeCID(fmt.Sprintf("commR: %d", i), &miner.SealedCIDPrefix)
		commD := tutil.MakeCID(fmt.Sprintf("commD: %d", i), &market.PieceCIDPrefix)

		dealCfg := &DealConfig{
			CommP:     commP,
			PieceSize: uint64(2048),
			Verified:  false,
			EndEpoch:  int64(538000),
		}

		cfgs[i] = &CommitConfig{
			CommR:     commR,
			CommD:     commD,
			SectorNum: abi.SectorNumber(i),
			DealCfg:   dealCfg,
			ProofType: constants.DevRegisteredSealProof,
		}
	}
	return cfgs, nil
}
