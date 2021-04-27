package consensusfault

import (
	"bytes"
	"context"
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/pkg/types"

	"github.com/filecoin-project/venus/pkg/config"
	"golang.org/x/xerrors"

	runtime2 "github.com/filecoin-project/specs-actors/v2/actors/runtime"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/state"
)

type FaultStateView interface {
	ResolveToKeyAddr(ctx context.Context, address address.Address) (address.Address, error)
	MinerInfo(ctx context.Context, maddr address.Address, nv network.Version) (*miner.MinerInfo, error)
}

// Chain state required for checking consensus fault reports.
type chainReader interface {
	GetTipSet(types.TipSetKey) (*types.TipSet, error)
}

// Checks the validity of reported consensus faults.
type ConsensusFaultChecker struct {
	chain chainReader
	fork  fork.IFork
}

func NewFaultChecker(chain chainReader, fork fork.IFork) *ConsensusFaultChecker {
	return &ConsensusFaultChecker{chain: chain, fork: fork}
}

// Checks the validity of a consensus fault reported by serialized block headers h1, h2, and optional
// common-ancestor witness h3.
func (s *ConsensusFaultChecker) VerifyConsensusFault(ctx context.Context, h1, h2, extra []byte, view FaultStateView) (*runtime2.ConsensusFault, error) {
	if bytes.Equal(h1, h2) {
		return nil, fmt.Errorf("no consensus fault: blocks identical")
	}

	var b1, b2, b3 types.BlockHeader
	innerErr := b1.UnmarshalCBOR(bytes.NewReader(h1))
	if innerErr != nil {
		return nil, errors.Wrapf(innerErr, "failed to decode h1")
	}
	innerErr = b2.UnmarshalCBOR(bytes.NewReader(h2))
	if innerErr != nil {
		return nil, errors.Wrapf(innerErr, "failed to decode h2")
	}

	// workaround chain halt
	forkUpgrade := s.fork.GetForkUpgrade()
	if config.IsNearUpgrade(b1.Height, forkUpgrade.UpgradeOrangeHeight) {
		return nil, xerrors.Errorf("consensus reporting disabled around Upgrade Orange")
	}
	if config.IsNearUpgrade(b2.Height, forkUpgrade.UpgradeOrangeHeight) {
		return nil, xerrors.Errorf("consensus reporting disabled around Upgrade Orange")
	}

	// BlockHeader syntax is not validated. This implements the strictest check possible, and is also the simplest check
	// possible.
	// This means that blocks that could never have been included in the chain (e.g. with an empty parent state)
	// are still fault-able.

	if b1.Miner != b2.Miner {
		return nil, fmt.Errorf("no consensus fault: miners differ")
	}
	if b1.Height > b2.Height {
		return nil, fmt.Errorf("no consensus fault: first block is higher than second")
	}

	// Check the basic fault conditions first, defer the (expensive) signature and chain history check until last.
	var fault *runtime2.ConsensusFault

	// Double-fork mining fault: two blocks at the same epoch.
	// It is not necessary to present a common ancestor of the blocks.
	if b1.Height == b2.Height {
		fault = &runtime2.ConsensusFault{
			Target: b1.Miner,
			Epoch:  b2.Height,
			Type:   runtime2.ConsensusFaultDoubleForkMining,
		}
	}
	// Time-offset mining fault: two blocks with the same parent but different epochs.
	// The height check is redundant at time of writing, but included for robustness to future changes to this method.
	// The blocks have a common ancestor by definition (the parent).
	if b1.Parents.Equals(b2.Parents) && b1.Height != b2.Height {
		fault = &runtime2.ConsensusFault{
			Target: b1.Miner,
			Epoch:  b2.Height,
			Type:   runtime2.ConsensusFaultTimeOffsetMining,
		}
	}
	// Parent-grinding fault: one block’s parent is a tipset that provably should have included some block but does not.
	// The provable case is that two blocks are mined and the later one does not include the
	// earlier one as a parent even though it could have.
	// B3 must prove that the higher block (B2) could have been included in B1's tipset.
	if len(extra) > 0 {
		innerErr = b3.UnmarshalCBOR(bytes.NewReader(extra))
		if innerErr != nil {
			return nil, errors.Wrapf(innerErr, "failed to decode extra")
		}
		if b1.Height == b3.Height && b3.Parents.Equals(b1.Parents) && !b2.Parents.Has(b1.Cid()) && b2.Parents.Has(b3.Cid()) {
			fault = &runtime2.ConsensusFault{
				Target: b1.Miner,
				Epoch:  b2.Height,
				Type:   runtime2.ConsensusFaultParentGrinding,
			}
		}
	}

	if fault == nil {
		return nil, fmt.Errorf("no consensus fault: blocks are ok")
	}

	// Expensive validation: signatures.
	b1Version := s.fork.GetNtwkVersion(ctx, b1.Height)
	err := verifyBlockSignature(ctx, view, b1, b1Version)
	if err != nil {
		return nil, err
	}
	b2Version := s.fork.GetNtwkVersion(ctx, b2.Height)
	err = verifyBlockSignature(ctx, view, b2, b2Version)
	if err != nil {
		return nil, err
	}

	return fault, nil
}

// Checks whether a block header is correctly signed in the context of the parent state to which it refers.
func verifyBlockSignature(ctx context.Context, view FaultStateView, blk types.BlockHeader, nv network.Version) error {
	minerInfo, err := view.MinerInfo(ctx, blk.Miner, nv)
	if err != nil {
		panic(errors.Wrapf(err, "failed to inspect miner addresses"))
	}
	if blk.BlockSig == nil {
		return errors.Errorf("no consensus fault: block %s has nil signature", blk.Cid())
	}
	err = state.NewSignatureValidator(view).ValidateSignature(ctx, blk.SignatureData(), minerInfo.Worker, *blk.BlockSig)
	if err != nil {
		return errors.Wrapf(err, "no consensus fault: block %s signature invalid", blk.Cid())
	}
	return err
}
