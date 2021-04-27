package vmcontext

import (
	"context"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/venus/pkg/chain"
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

type ExecCallBack func(cid.Cid, VmMessage, *Ret) error
type CircSupplyCalculator func(context.Context, abi.ChainEpoch, tree.Tree) (abi.TokenAmount, error)
type NtwkVersionGetter func(context.Context, abi.ChainEpoch) network.Version

type VmOption struct { //nolint
	CircSupplyCalculator CircSupplyCalculator
	NtwkVersionGetter    NtwkVersionGetter
	Rnd                  chain.RandomnessSource
	BaseFee              abi.TokenAmount
	Fork                 fork.IFork
	ActorCodeLoader      *dispatch.CodeLoader
	Epoch                abi.ChainEpoch
	GasPriceSchedule     *gas.PricesSchedule
	PRoot                cid.Cid
	Bsstore              blockstoreutil.Blockstore
	SysCallsImpl         SyscallsImpl
}

type Ret struct {
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
