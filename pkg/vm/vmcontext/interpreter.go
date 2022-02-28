package vmcontext

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/venus-shared/types"
)

// VMInterpreter orchestrates the execution of messages from a tipset on that tipset’s parent State.
type VMInterpreter interface {
	// Note: any message processing error will be present as an `ExitCode` in the `MessageReceipt`.
	ApplyGenesisMessage(from address.Address, to address.Address, method abi.MethodNum, value abi.TokenAmount, params interface{}) (*Ret, error)
	ApplyMessage(ctx context.Context, msg types.ChainMsg) (*Ret, error)
	ApplyImplicitMessage(ctx context.Context, msg types.ChainMsg) (*Ret, error)

	StateTree() tree.Tree
	Flush(ctx context.Context) (tree.Root, error)
}
