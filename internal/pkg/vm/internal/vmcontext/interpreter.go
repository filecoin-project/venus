package vmcontext

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/specactors/adt"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

// VMInterpreter orchestrates the execution of messages from a tipset on that tipset’s parent state.
type VMInterpreter interface {
	// ApplyTipSetMessages applies all the messages in a tipset.
	//
	// Note: any message processing error will be present as an `ExitCode` in the `MessageReceipt`.
	ApplyTipSetMessages(blocks []BlockMessagesInfo, ts *block.TipSet, parentEpoch abi.ChainEpoch, epoch abi.ChainEpoch, cb ExecCallBack) ([]types.MessageReceipt, error)
	ContextStore() adt.Store
	ApplyMessage(msg types.ChainMsg) *Ret
}
