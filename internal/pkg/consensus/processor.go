package consensus

import (
	"context"
	"math/big"

	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// MessageValidator validates the syntax and semantics of a message before it is applied.
type MessageValidator interface {
	// Validate checks a message for validity.
	Validate(ctx context.Context, msg *types.UnsignedMessage, fromActor *actor.Actor) error
}

// ApplicationResult contains the result of successfully applying one message.
// ExecutionError might be set and the message can still be applied successfully.
// See ApplyMessage() for details.
type ApplicationResult struct {
	Receipt        *types.MessageReceipt
	ExecutionError error
}

// ApplyMessageResult is the result of applying a single message.
type ApplyMessageResult struct {
	ApplicationResult        // Application-level result, if error is nil.
	Failure            error // Failure to apply the message
	FailureIsPermanent bool  // Whether failure is permanent, has no chance of succeeding later.
}

// DefaultProcessor handles all block processing.
type DefaultProcessor struct {
	validator MessageValidator
	actors    builtin.Actors
}

var _ Processor = (*DefaultProcessor)(nil)

// NewDefaultProcessor creates a default processor from the given state tree and vms.
func NewDefaultProcessor() *DefaultProcessor {
	return &DefaultProcessor{
		validator: NewDefaultMessageValidator(),
		actors:    builtin.DefaultActors,
	}
}

// NewConfiguredProcessor creates a default processor with custom validation and rewards.
func NewConfiguredProcessor(validator MessageValidator, actors builtin.Actors) *DefaultProcessor {
	return &DefaultProcessor{
		validator: validator,
		actors:    actors,
	}
}

// ProcessTipSet computes the state transition specified by the messages in all
// blocks in a TipSet.  It is similar to ProcessBlock with a few key differences.
// Most importantly ProcessTipSet relies on the precondition that each input block
// is valid with respect to the base state st, that is, ProcessBlock is free of
// errors when applied to each block individually over the given state.
// ProcessTipSet only returns errors in the case of faults.  Other errors
// coming from calls to ApplyMessage can be traced to different blocks in the
// TipSet containing conflicting messages and are returned in the result slice.
// Blocks are applied in the sorted order of their tickets.
func (p *DefaultProcessor) ProcessTipSet(ctx context.Context, st state.Tree, vms vm.Storage, ts block.TipSet, msgs []vm.BlockMessagesInfo) (results []vm.MessageReceipt, err error) {
	ctx, span := trace.StartSpan(ctx, "DefaultProcessor.ProcessTipSet")
	span.AddAttributes(trace.StringAttribute("tipset", ts.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	h, err := ts.Height()
	if err != nil {
		return nil, errors.FaultErrorWrap(err, "processing empty tipset")
	}
	epoch := types.NewBlockHeight(h)

	vm := vm.NewVM(st, &vms)

	return vm.ApplyTipSetMessages(msgs, *epoch)
}

// CallQueryMethod calls a method on an actor in the given state tree. It does
// not make any changes to the state/blockchain and is useful for interrogating
// actor state. Block height bh is optional; some methods will ignore it.
func (p *DefaultProcessor) CallQueryMethod(ctx context.Context, st state.Tree, vms vm.StorageMap, to address.Address, method types.MethodID, params []byte, from address.Address, optBh *types.BlockHeight) ([][]byte, uint8, error) {
	// not committing or flushing storage structures guarantees changes won't make it to stored state tree or datastore
	cachedSt := state.NewCachedTree(st)

	msg := &types.UnsignedMessage{
		From:       from,
		To:         to,
		CallSeqNum: 0,
		Value:      types.ZeroAttoFIL,
		Method:     method,
		Params:     params,
	}

	// Set the gas limit to the max because this message send should always succeed; it doesn't cost gas.
	gasTracker := vm.NewLegacyGasTracker()
	gasTracker.MsgGasLimit = types.BlockGasLimit

	// translate address before retrieving from actor
	toAddr, found, err := ResolveAddress(ctx, msg.To, cachedSt, vms, gasTracker)
	if err != nil {
		return nil, 1, errors.FaultErrorWrapf(err, "Could not resolve actor address")
	}

	if !found {
		return nil, 1, errors.ApplyErrorPermanentWrapf(err, "failed to resolve To actor")
	}

	toActor, err := st.GetActor(ctx, toAddr)
	if err != nil {
		return nil, 1, errors.ApplyErrorPermanentWrapf(err, "failed to get To actor")
	}

	vmCtxParams := vm.NewContextParams{
		To:          toActor,
		ToAddr:      toAddr,
		Message:     msg,
		OriginMsg:   msg,
		State:       cachedSt,
		StorageMap:  vms,
		GasTracker:  gasTracker,
		BlockHeight: optBh,
		Actors:      p.actors,
	}

	vmCtx := vm.NewVMContext(vmCtxParams)
	ret, retCode, err := vm.Send(ctx, vmCtx)
	return ret, retCode, err
}

// PreviewQueryMethod estimates the amount of gas that will be used by a method
// call. It accepts all the same arguments as CallQueryMethod.
func (p *DefaultProcessor) PreviewQueryMethod(ctx context.Context, st state.Tree, vms vm.StorageMap, to address.Address, method types.MethodID, params []byte, from address.Address, optBh *types.BlockHeight) (types.GasUnits, error) {
	// not committing or flushing storage structures guarantees changes won't make it to stored state tree or datastore
	cachedSt := state.NewCachedTree(st)

	msg := &types.UnsignedMessage{
		From:       from,
		To:         to,
		CallSeqNum: 0,
		Value:      types.ZeroAttoFIL,
		Method:     method,
		Params:     params,
	}

	// Set the gas limit to the max because this message send should always succeed; it doesn't cost gas.
	gasTracker := vm.NewLegacyGasTracker()
	gasTracker.MsgGasLimit = types.BlockGasLimit

	// ensure actor exists
	toActor, toAddr, err := getOrCreateActor(ctx, cachedSt, vms, msg.To, gasTracker)
	if err != nil {
		return types.GasUnits(0), errors.FaultErrorWrap(err, "failed to get To actor")
	}

	vmCtxParams := vm.NewContextParams{
		To:          toActor,
		ToAddr:      toAddr,
		Message:     msg,
		OriginMsg:   msg,
		State:       cachedSt,
		StorageMap:  vms,
		GasTracker:  gasTracker,
		BlockHeight: optBh,
		Actors:      p.actors,
	}
	vmCtx := vm.NewVMContext(vmCtxParams)
	_, _, err = vm.Send(ctx, vmCtx)

	return vmCtx.GasUnits(), err
}

// ResolveAddress looks up associated id address. If the given address is already and id address, it is returned unchanged.
func ResolveAddress(ctx context.Context, addr address.Address, st *state.CachedTree, vms vm.StorageMap, gt *vm.LegacyGasTracker) (address.Address, bool, error) {
	if addr.Protocol() == address.ID {
		return addr, true, nil
	}

	init, err := st.GetActor(ctx, address.InitAddress)
	if err != nil {
		return address.Undef, false, err
	}

	vmCtx := vm.NewVMContext(vm.NewContextParams{
		State:      st,
		StorageMap: vms,
		ToAddr:     address.InitAddress,
		To:         init,
	})

	id, found, err := initactor.LookupIDAddress(vmCtx, addr)
	if err != nil {
		return address.Undef, false, err
	}

	if !found {
		return address.Undef, false, nil
	}

	idAddr, err := address.NewIDAddress(id)
	if err != nil {
		return address.Undef, false, err
	}

	return idAddr, true, nil
}

func getOrCreateActor(ctx context.Context, st *state.CachedTree, store vm.StorageMap, addr address.Address, gt *vm.LegacyGasTracker) (*actor.Actor, address.Address, error) {
	// resolve address before lookup
	idAddr, found, err := ResolveAddress(ctx, addr, st, store, gt)
	if err != nil {
		return nil, address.Undef, err
	}

	if found {
		act, err := st.GetActor(ctx, idAddr)
		return act, idAddr, err
	}

	initAct, err := st.GetActor(ctx, address.InitAddress)
	if err != nil {
		return nil, address.Undef, err
	}

	// this should never fail due to lack of gas since gas doesn't have meaning here
	noopGT := vm.NewLegacyGasTracker()
	noopGT.MsgGasLimit = 10000 // must exceed gas units consumed by init.Exec+account.Constructor+init.GetActorIDForAddress
	vmctx := vm.NewVMContext(vm.NewContextParams{Actors: builtin.DefaultActors, To: initAct, State: st, StorageMap: store, GasTracker: noopGT})
	vmctx.Send(address.InitAddress, initactor.ExecMethodID, types.ZeroAttoFIL, []interface{}{types.AccountActorCodeCid, []interface{}{addr}})

	vmctx = vm.NewVMContext(vm.NewContextParams{Actors: builtin.DefaultActors, To: initAct, State: st, StorageMap: store, GasTracker: noopGT})
	idAddrInt := vmctx.Send(address.InitAddress, initactor.GetActorIDForAddressMethodID, types.ZeroAttoFIL, []interface{}{addr})

	id, ok := idAddrInt.(*big.Int)
	if !ok {
		return nil, address.Undef, errors.NewFaultError("non-integer return from GetActorIDForAddress")
	}

	idAddr, err = address.NewIDAddress(id.Uint64())
	if err != nil {
		return nil, address.Undef, err
	}

	act, err := st.GetActor(ctx, idAddr)
	return act, idAddr, err
}
