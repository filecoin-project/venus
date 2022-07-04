package consensusfault

import (
	"bytes"
	"context"
	"fmt"

	cbornode "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	runtime7 "github.com/filecoin-project/specs-actors/v7/actors/runtime"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	"github.com/filecoin-project/venus/venus-shared/actors/policy"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type FaultStateView interface {
	ResolveToKeyAddr(ctx context.Context, address address.Address) (address.Address, error)
	MinerInfo(ctx context.Context, maddr address.Address, nv network.Version) (*miner.MinerInfo, error)
}

// Chain state required for checking consensus fault reports.
type chainReader interface {
	GetTipSet(context.Context, types.TipSetKey) (*types.TipSet, error)
}

// Checks the validity of reported consensus faults.
type ConsensusFaultChecker struct { //nolint
	chain chainReader
	fork  fork.IFork
}

func NewFaultChecker(chain chainReader, fork fork.IFork) *ConsensusFaultChecker {
	return &ConsensusFaultChecker{chain: chain, fork: fork}
}

// Checks validity of the submitted consensus fault with the two block headers needed to prove the fault
// and an optional extra one to check common ancestry (as needed).
// Note that the blocks are ordered: the method requires a.Epoch() <= b.Epoch().
func (s *ConsensusFaultChecker) VerifyConsensusFault(ctx context.Context, h1, h2, extra []byte, curEpoch abi.ChainEpoch, msg vm.VmMessage, gasIpld cbornode.IpldStore, view vm.SyscallsStateView, getter vmcontext.LookbackStateGetter) (*runtime7.ConsensusFault, error) {
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
		return nil, fmt.Errorf("consensus reporting disabled around Upgrade Orange")
	}
	if config.IsNearUpgrade(b2.Height, forkUpgrade.UpgradeOrangeHeight) {
		return nil, fmt.Errorf("consensus reporting disabled around Upgrade Orange")
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
	var fault *runtime7.ConsensusFault

	// Double-fork mining fault: two blocks at the same epoch.
	// It is not necessary to present a common ancestor of the blocks.
	if b1.Height == b2.Height {
		fault = &runtime7.ConsensusFault{
			Target: b1.Miner,
			Epoch:  b2.Height,
			Type:   runtime7.ConsensusFaultDoubleForkMining,
		}
	}
	// Time-offset mining fault: two blocks with the same parent but different epochs.
	// The curEpoch check is redundant at time of writing, but included for robustness to future changes to this method.
	// The blocks have a common ancestor by definition (the parent).
	b1PKey := types.NewTipSetKey(b1.Parents...)
	b2PKey := types.NewTipSetKey(b2.Parents...)
	if b1PKey.Equals(b2PKey) && b1.Height != b2.Height {
		fault = &runtime7.ConsensusFault{
			Target: b1.Miner,
			Epoch:  b2.Height,
			Type:   runtime7.ConsensusFaultTimeOffsetMining,
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
		b3PKey := types.NewTipSetKey(b3.Parents...)
		if b1.Height == b3.Height && b3PKey.Equals(b1PKey) && !b2PKey.Has(b1.Cid()) && b2PKey.Has(b3.Cid()) {
			fault = &runtime7.ConsensusFault{
				Target: b1.Miner,
				Epoch:  b2.Height,
				Type:   runtime7.ConsensusFaultParentGrinding,
			}
		}
	}

	if fault == nil {
		return nil, fmt.Errorf("no consensus fault: blocks are ok")
	}

	// Expensive validation: signatures.
	b1Version := s.fork.GetNetworkVersion(ctx, b1.Height)
	err := verifyBlockSignature(ctx, b1, b1Version, curEpoch, msg.To, gasIpld, view, getter)
	if err != nil {
		return nil, err
	}
	b2Version := s.fork.GetNetworkVersion(ctx, b2.Height)
	err = verifyBlockSignature(ctx, b2, b2Version, curEpoch, msg.To, gasIpld, view, getter)
	if err != nil {
		return nil, err
	}

	return fault, nil
}

// Checks whether a block header is correctly signed in the context of the parent state to which it refers.
func verifyBlockSignature(ctx context.Context, blk types.BlockHeader, nv network.Version, curEpoch abi.ChainEpoch, receiver address.Address, gasIpld cbornode.IpldStore, view FaultStateView, getter vmcontext.LookbackStateGetter) error {
	if nv >= network.Version7 && blk.Height < curEpoch-policy.ChainFinality {
		return fmt.Errorf("cannot get worker key (currEpoch %d, height %d)", curEpoch, blk.Height)
	}

	lbstate, err := getter(ctx, blk.Height)
	if err != nil {
		return fmt.Errorf("fialed to look back state at height %d", blk.Height)
	}

	act, err := lbstate.LoadActor(ctx, receiver)
	if err != nil {
		return errors.Wrapf(err, "failed to get miner actor")
	}

	mas, err := miner.Load(adt.WrapStore(ctx, gasIpld), act)
	if err != nil {
		return fmt.Errorf("failed to load state for miner %s", receiver)
	}

	info, err := mas.Info()
	if err != nil {
		return fmt.Errorf("failed to get miner info for miner %s", receiver)
	}

	if blk.BlockSig == nil {
		return errors.Errorf("no consensus fault: block %s has nil signature", blk.Cid())
	}

	sd, err := blk.SignatureData()
	if err != nil {
		return err
	}
	err = state.NewSignatureValidator(view).ValidateSignature(ctx, sd, info.Worker, *blk.BlockSig)
	if err != nil {
		return errors.Wrapf(err, "no consensus fault: block %s signature invalid", blk.Cid())
	}
	return err
}
