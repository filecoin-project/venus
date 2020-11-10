package gengen

import (
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
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
		commP, err := constants.DefaultCidBuilder.Sum([]byte(fmt.Sprintf("commP: %d", i)))
		if err != nil {
			return nil, err
		}
		commR, err := constants.DefaultCidBuilder.Sum([]byte(fmt.Sprintf("commR: %d", i)))
		if err != nil {
			return nil, err
		}
		commD, err := constants.DefaultCidBuilder.Sum([]byte(fmt.Sprintf("commD: %d", i)))
		if err != nil {
			return nil, err
		}

		dealCfg := &DealConfig{
			CommP:     commP,
			PieceSize: uint64(1),
			Verified:  false,
			EndEpoch:  int64(1024),
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
