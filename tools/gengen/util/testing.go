package gengen

import (
	"fmt"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	miner0 "github.com/filecoin-project/specs-actors/actors/builtin/miner"
	tutil "github.com/filecoin-project/specs-actors/support/testing"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbornode "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/constants"
	"github.com/filecoin-project/venus/internal/pkg/version"
)

// MakeCommitCfgs creates n gengen commit configs, casting strings to cids.
func MakeCommitCfgs(n int) ([]*CommitConfig, error) {
	cfgs := make([]*CommitConfig, n)
	for i := 0; i < n; i++ {
		commP := tutil.MakeCID(fmt.Sprintf("commP: %d", i), &market.PieceCIDPrefix)
		commR := tutil.MakeCID(fmt.Sprintf("commR: %d", i), &miner0.SealedCIDPrefix)
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

// DefaultGenesis creates a test network genesis block with default accounts and actors installed.
func DefaultGenesis(cst cbornode.IpldStore, bs blockstore.Blockstore) (*block.Block, error) {
	return MakeGenesisFunc(NetworkName(version.TEST))(cst, bs)
}
