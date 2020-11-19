package vmcontext

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm/dispatch"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/pkg/vm/state"
)

type ExecCallBack func(cid.Cid, VmMessage, *Ret) error
type CircSupplyCalculator func(context.Context, abi.ChainEpoch, state.Tree) (abi.TokenAmount, error)
type NtwkVersionGetter func(context.Context, abi.ChainEpoch) network.Version

// BlockMessagesInfo contains messages for one block in a tipset.
type BlockMessagesInfo struct {
	BLSMessages  []*types.UnsignedMessage
	SECPMessages []*types.SignedMessage
	Miner        address.Address
	WinCount     int64
}

type VmOption struct { //nolint
	CircSupplyCalculator CircSupplyCalculator
	NtwkVersionGetter    NtwkVersionGetter
	Rnd                  crypto.RandomnessSource
	BaseFee              abi.TokenAmount
	Fork                 fork.IFork
	ActorCodeLoader      *dispatch.CodeLoader
	Epoch                abi.ChainEpoch
}

type Ret struct {
	GasTracker *gas.GasTracker
	OutPuts    gas.GasOutputs
	Receipt    types.MessageReceipt
}
