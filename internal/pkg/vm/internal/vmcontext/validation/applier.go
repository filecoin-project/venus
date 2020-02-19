package validation

import (
	"context"

	vtypes "github.com/filecoin-project/chain-validation/chain/types"
	vstate "github.com/filecoin-project/chain-validation/state"
	abi_spec "github.com/filecoin-project/specs-actors/actors/abi"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storage"
	vmctx "github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/vmcontext"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// Applier applies messages to state trees and storage.
type Applier struct {
}

var _ vstate.Applier = &Applier{}

func NewApplier() *Applier {
	return &Applier{}
}

func (a *Applier) ApplyMessage(eCtx *vtypes.ExecutionContext, wrapper vstate.Wrapper, message *vtypes.Message) (vtypes.MessageReceipt, error) {
	ctx := context.TODO()
	st := wrapper.(*StateWrapper)

	randSrc := &vmRand{}

	tree, err := state.NewTreeLoader().LoadStateTree(ctx, st.storage, st.stateRoot)
	if err != nil {
		panic(err)
	}
	vmStrg := storage.NewStorage(st.bs)
	gfcVm := vmctx.NewVM(randSrc, builtin.DefaultActors, &vmStrg, tree)

	// NB: Not using ApplyGenesisMessage here because we need gas tracking and message receipts - ApplyMessage provides these.
	// NB: at this stage we only need the receipt and don't care about the miner penalty - second return value.
	receipt, _ := gfcVm.ApplyMessage(togfcMsg(message), 0, eCtx.MinerOwner)

	// FIXME not sure what to flush, or in which order
	// TODO remove panic when the above fixme is addressed
	if err := vmStrg.Flush(); err != nil {
		panic(err)
	}

	// TODO remove panic when the above fixme is addressed
	st.stateRoot, err = tree.Flush(ctx)
	if err != nil {
		panic(err)
	}

	return vtypes.MessageReceipt{
		ExitCode:    receipt.ExitCode,
		ReturnValue: receipt.ReturnValue,
		GasUsed:     receipt.GasUsed.AsBigInt(),
	}, nil
}

type vmRand struct{}

func (*vmRand) Randomness(epoch abi_spec.ChainEpoch) abi_spec.Randomness {
	panic("implement me")
}

func togfcMsg(msg *vtypes.Message) *types.UnsignedMessage {
	return &types.UnsignedMessage{
		To:   msg.To,
		From: msg.From,

		CallSeqNum: uint64(msg.CallSeqNum),
		Method:     types.MethodID(msg.Method),

		Value:    msg.Value,
		GasPrice: msg.GasPrice,
		GasLimit: types.GasUnits(msg.GasLimit.Uint64()),

		Params: msg.Params,
	}
}
