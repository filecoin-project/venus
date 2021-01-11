package vmcontext

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm/state"
	"github.com/ipfs/go-cid"
)

// VMInterpreter orchestrates the execution of messages from a tipset on that tipset’s parent State.
type VMInterpreter interface {
	// ApplyTipSetMessages applies all the messages in a tipset.
	//
	// Note: any message processing error will be present as an `ExitCode` in the `MessageReceipt`.
	ApplyTipSetMessages(blocks []block.BlockMessagesInfo, ts *block.TipSet, parentEpoch abi.ChainEpoch, epoch abi.ChainEpoch, cb ExecCallBack) (cid.Cid, []types.MessageReceipt, error)
	ApplyGenesisMessage(from address.Address, to address.Address, method abi.MethodNum, value abi.TokenAmount, params interface{}) (*Ret, error)
	ApplyMessage(msg types.ChainMsg) *Ret
	ApplyImplicitMessage(msg types.ChainMsg) (*Ret, error)

	StateTree() state.Tree
	Flush() (state.Root, error)

	MutateState(ctx context.Context, addr address.Address, fn interface{}) error
}
