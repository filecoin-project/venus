package vmcontext

import (
	"context"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm/dispatch"
	"github.com/filecoin-project/venus/pkg/vm/gas"
)

type ExecCallBack func(cid.Cid, *types.UnsignedMessage, *Ret) error
type CircSupplyCalculator func(context.Context, abi.ChainEpoch, tree.Tree) (abi.TokenAmount, error)
type NtwkVersionGetter func(context.Context, abi.ChainEpoch) network.Version
type LookbackStateGetter func(context.Context, abi.ChainEpoch) (*state.View, error)

type VmOption struct { //nolint
	CircSupplyCalculator CircSupplyCalculator
	LookbackStateGetter  LookbackStateGetter
	NtwkVersionGetter    NtwkVersionGetter
	Rnd                  HeadChainRandomness
	BaseFee              abi.TokenAmount
	Fork                 fork.IFork
	ActorCodeLoader      *dispatch.CodeLoader
	Epoch                abi.ChainEpoch
	GasPriceSchedule     *gas.PricesSchedule
	PRoot                cid.Cid
	Bsstore              blockstoreutil.Blockstore
	SysCallsImpl         SyscallsImpl

	RecordTraces bool
}

//ChainRandomness define randomness method in filecoin
type ILookBack interface {
	StateView(ts *types.TipSet) (*state.View, error)
	GetLookbackTipSetForRound(ctx context.Context, ts *types.TipSet, round abi.ChainEpoch, version network.Version) (*types.TipSet, cid.Cid, error)
}

func LookbackStateGetterForTipset(backer ILookBack, fork fork.IFork, ts *types.TipSet) LookbackStateGetter {
	return func(ctx context.Context, round abi.ChainEpoch) (*state.View, error) {
		ver := fork.GetNtwkVersion(ctx, round)
		ts, _, err := backer.GetLookbackTipSetForRound(ctx, ts, round, ver)
		if err != nil {
			return nil, err
		}
		return backer.StateView(ts)
	}
}

//ChainRandomness define randomness method in filecoin
type HeadChainRandomness interface {
	ChainGetRandomnessFromBeacon(ctx context.Context, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
	ChainGetRandomnessFromTickets(ctx context.Context, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
}

type Ret struct {
	RootCid    cid.Cid
	GasTracker *gas.GasTracker
	OutPuts    gas.GasOutputs
	Receipt    types.MessageReceipt
}

// Failure returns with a non-zero exit code.
func Failure(exitCode exitcode.ExitCode, gasAmount int64) types.MessageReceipt {
	return types.MessageReceipt{
		ExitCode:    exitCode,
		ReturnValue: []byte{},
		GasUsed:     gasAmount,
	}
}
