package validation

import (
	"context"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"

	vchain "github.com/filecoin-project/chain-validation/pkg/chain"
	vstate "github.com/filecoin-project/chain-validation/pkg/state"

	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// Applier applies messages to state trees and storage.
type Applier struct {
	processor *consensus.DefaultProcessor
}

var _ vchain.Applier = &Applier{}

func NewApplier() *Applier {
	return &Applier{consensus.NewDefaultProcessor()}
}

func (a *Applier) ApplyMessage(eCtx *vchain.ExecutionContext, state vstate.Wrapper, message interface{}) (vchain.MessageReceipt, error) {
	ctx := context.TODO()
	stateTree := state.(*StateWrapper).Tree
	vms := state.(*StateWrapper).StorageMap
	msg := message.(*types.SignedMessage)
	minerOwner, err := address.NewFromBytes([]byte(eCtx.MinerOwner))
	if err != nil {
		return vchain.MessageReceipt{}, err
	}
	blockHeight := types.NewBlockHeight(eCtx.Epoch)
	gasTracker := vm.NewGasTracker()
	// Providing direct access to blockchain structures is very difficult and expected to go away.
	var ancestors []block.TipSet

	amr, err := a.processor.ApplyMessage(ctx, stateTree, vms, msg, minerOwner, blockHeight, gasTracker, ancestors)
	if err != nil {
		return vchain.MessageReceipt{}, err
	}
	// Go-filecoin has some messed-up nested array return value.
	retVal := []byte{}
	if len(amr.Receipt.Return) > 0 {
		retVal = amr.Receipt.Return[0]
	}
	mr := vchain.MessageReceipt{
		ExitCode:    amr.Receipt.ExitCode,
		ReturnValue: retVal,
		// Go-filecoin returns the gas cost rather than gas unit consumption :-(
		GasUsed: vstate.GasUnit(amr.Receipt.GasAttoFIL.AsBigInt().Uint64()),
	}

	return mr, nil
}
