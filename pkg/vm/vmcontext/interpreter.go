package vmcontext

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/state/tree"
)

// VMInterpreter orchestrates the execution of messages from a tipset on that tipset’s parent State.
type VMInterpreter interface {
	// Note: any message processing error will be present as an `ExitCode` in the `MessageReceipt`.
	ApplyGenesisMessage(from address.Address, to address.Address, method abi.MethodNum, value abi.TokenAmount, params interface{}) (*Ret, error)

	Interface

	StateTree() tree.Tree
}
