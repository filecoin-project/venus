package vmcontext

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
)

// VMInterpreter orchestrates the execution of messages from a tipset on that tipset’s parent State.
type VMInterpreter interface {
	// ApplyTipSetMessages applies all the messages in a tipset.
	//
	// Note: any message processing error will be present as an `ExitCode` in the `MessageReceipt`.
	ApplyTipSetMessages(blocks []types.BlockMessagesInfo, ts *types.TipSet, parentEpoch abi.ChainEpoch, epoch abi.ChainEpoch, cb ExecCallBack) (cid.Cid, []types.MessageReceipt, []types.BlockReward, error)
	ApplyGenesisMessage(from address.Address, to address.Address, method abi.MethodNum, value abi.TokenAmount, params interface{}) (*Ret, error)
	ApplyMessage(msg types.ChainMsg) (*Ret, error)
	ApplyImplicitMessage(msg types.ChainMsg) (*Ret, error)

	StateTree() tree.Tree
	Flush() (tree.Root, error)
}
