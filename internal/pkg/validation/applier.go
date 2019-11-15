package validation

import (
	"context"

	vchain "github.com/filecoin-project/chain-validation/pkg/chain"
	vstate "github.com/filecoin-project/chain-validation/pkg/state"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	fcstate "github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// Applier applies messages to state trees and storage.
type Applier struct {
	processor *consensus.DefaultProcessor
}

var _ vchain.Applier = &Applier{}

// NewApplier returns an Applier.
func NewApplier() *Applier {
	return &Applier{consensus.NewDefaultProcessor()}
}

// ApplyMessage applies a message to the state tree and returns the receipt of its application or an error.
func (a *Applier) ApplyMessage(eCtx *vchain.ExecutionContext, state vstate.Wrapper, message interface{}) (vchain.MessageReceipt, error) {
	ctx := context.TODO()

	wrapper := state.(*StateWrapper)
	vms := state.(*StateWrapper).StorageMap
	msg := message.(*types.UnsignedMessage)

	blockHeight := types.NewBlockHeight(eCtx.Epoch)
	gasTracker := vm.NewGasTracker()
	base := state.Cid()

	minerOwner, err := address.NewFromBytes([]byte(eCtx.MinerOwner))
	if err != nil {
		return vchain.MessageReceipt{}, err
	}

	tree, err := fcstate.NewTreeLoader().LoadStateTree(ctx, wrapper.cst, base)
	if err != nil {
		return vchain.MessageReceipt{}, err
	}

	// Providing direct access to blockchain structures is very difficult and expected to go away.
	var ancestors []block.TipSet
	amr, err := a.processor.ApplyMessage(ctx, tree, vms, msg, minerOwner, blockHeight, gasTracker, ancestors)
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

	wrapper.stateRoot, err = tree.Flush(ctx)
	if err != nil {
		return vchain.MessageReceipt{}, err
	}

	return mr, nil
}
