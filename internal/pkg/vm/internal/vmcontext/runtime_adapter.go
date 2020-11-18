package vmcontext

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	cbor2 "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/go-state-types/rt"
	rtt "github.com/filecoin-project/go-state-types/rt"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/venus/internal/pkg/enccid"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/internal/pkg/types"
	"github.com/filecoin-project/venus/internal/pkg/vm/gas"
	"github.com/filecoin-project/venus/internal/pkg/vm/internal/pattern"
	"github.com/filecoin-project/venus/internal/pkg/vm/internal/runtime"
)

var EmptyObjectCid cid.Cid

func init() {
	cst := cbor2.NewMemCborStore()
	emptyobject, err := cst.Put(context.TODO(), []struct{}{})
	if err != nil {
		panic(err)
	}

	EmptyObjectCid = emptyobject
}

var actorLog = logging.Logger("vm.actors")

var _ specsruntime.Runtime = (*runtimeAdapter)(nil)

type runtimeAdapter struct {
	ctx *invocationContext
	syscalls
}

func newRuntimeAdapter(ctx *invocationContext) *runtimeAdapter {
	return &runtimeAdapter{ctx: ctx, syscalls: syscalls{
		impl:      ctx.vm.syscalls,
		ctx:       ctx.vm.context,
		gasTank:   ctx.gasTank,
		pricelist: ctx.vm.pricelist,
		stateView: ctx.stateView(),
	}}
}

func (a *runtimeAdapter) Caller() address.Address {
	/*	if a.ctx.Message().Caller().Protocol() != address.ID {
		panic("runtime message has a non-ID caller")
	}*/
	return a.ctx.Message().Caller()
}

func (a *runtimeAdapter) Receiver() address.Address {
	//todo refer lotus imple
	/*	if a.ctx.Message().Receiver() != address.Undef && a.ctx.Message().Receiver().Protocol() != address.ID {
		panic("runtime message has a non-ID receiver")
	}*/
	return a.ctx.Message().Receiver()
}

func (a *runtimeAdapter) ValueReceived() abi.TokenAmount {
	return a.ctx.Message().ValueReceived()
}

func (a *runtimeAdapter) StateCreate(obj cbor.Marshaler) {
	c := a.StorePut(obj)
	err := a.stateCommit(EmptyObjectCid, c)
	if err != nil {
		panic(fmt.Errorf("failed To commit stateView after creating object: %w", err))
	}
}

func (a *runtimeAdapter) stateCommit(oldh, newh cid.Cid) error {

	// TODO: we can make this more efficient in the future...
	act, found, err := a.ctx.vm.state.GetActor(a.Context(), a.Receiver())
	if !found || err != nil {
		return xerrors.Errorf("failed To get actor To commit stateView, %s", err)
	}

	if act.Head.Cid != oldh {
		return xerrors.Errorf("failed To update, inconsistent base reference, %s", err)
	}

	act.Head = enccid.NewCid(newh)
	if err := a.ctx.vm.state.SetActor(a.Context(), a.Receiver(), act); err != nil {
		return xerrors.Errorf("failed To set actor in commit stateView, %s", err)
	}

	return nil
}

func (a *runtimeAdapter) StateReadonly(obj cbor.Unmarshaler) {
	act, found, err := a.ctx.vm.state.GetActor(a.Context(), a.Receiver())
	if !found || err != nil {
		a.Abortf(exitcode.SysErrorIllegalArgument, "failed To get actor for Readonly stateView: %s", err)
	}
	a.StoreGet(act.Head.Cid, obj)
}

func (a *runtimeAdapter) StateTransaction(obj cbor.Er, f func()) {
	if obj == nil {
		a.Abortf(exitcode.SysErrorIllegalActor, "Must not pass nil To Transaction()")
	}

	act, found, err := a.ctx.vm.state.GetActor(a.Context(), a.Receiver())
	if !found || err != nil {
		a.Abortf(exitcode.SysErrorIllegalActor, "failed To get actor for Transaction: %s", err)
	}
	baseState := act.Head.Cid
	a.StoreGet(baseState, obj)

	a.ctx.allowSideEffects = false
	f()
	a.ctx.allowSideEffects = true

	c := a.StorePut(obj)

	err = a.stateCommit(baseState, c)
	if err != nil {
		panic(fmt.Errorf("failed To commit stateView after transaction: %w", err))
	}
}

func (a *runtimeAdapter) StoreGet(c cid.Cid, o cbor.Unmarshaler) bool {
	return a.ctx.Store().StoreGet(c, o)
}

func (a *runtimeAdapter) StorePut(x cbor.Marshaler) cid.Cid {
	return a.ctx.Store().StorePut(x)
}

func (a *runtimeAdapter) NetworkVersion() network.Version {
	return a.stateView.GetNtwkVersion(a.Context(), a.CurrEpoch())
}

func (a *runtimeAdapter) GetRandomnessFromBeacon(personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) abi.Randomness {
	res, err := a.ctx.randSource.GetRandomnessFromBeacon(a.Context(), personalization, randEpoch, entropy)
	if err != nil {
		panic(xerrors.Errorf("could not get randomness: %s", err))
	}
	return res
}

func (a *runtimeAdapter) GetRandomnessFromTickets(personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) abi.Randomness {
	res, err := a.ctx.randSource.Randomness(a.Context(), personalization, randEpoch, entropy)
	if err != nil {
		panic(xerrors.Errorf("could not get randomness: %s", err))
	}
	return res
}

func (a *runtimeAdapter) Send(toAddr address.Address, methodNum abi.MethodNum, params cbor.Marshaler, value abi.TokenAmount, out cbor.Er) exitcode.ExitCode {
	return a.ctx.Send(toAddr, methodNum, params, value, out)
}

func (a *runtimeAdapter) ChargeGas(name string, compute int64, virtual int64) {
	a.gasTank.Charge(gas.NewGasCharge(name, compute, 0).WithVirtual(virtual, 0), "runtimeAdapter charge gas")
}

func (a *runtimeAdapter) Log(level rt.LogLevel, msg string, args ...interface{}) {
	switch level {
	case rtt.DEBUG:
		actorLog.Debugf(msg, args...)
	case rtt.INFO:
		actorLog.Infof(msg, args...)
	case rtt.WARN:
		actorLog.Warnf(msg, args...)
	case rtt.ERROR:
		actorLog.Errorf(msg, args...)
	}
}

// Message implements Runtime.
func (a *runtimeAdapter) Message() specsruntime.Message {
	return a.ctx.Message()
}

// CurrEpoch implements Runtime.
func (a *runtimeAdapter) CurrEpoch() abi.ChainEpoch {
	return a.ctx.Runtime().CurrentEpoch()
}

// ImmediateCaller implements Runtime.
func (a *runtimeAdapter) ImmediateCaller() address.Address {
	return a.ctx.Message().Caller()
}

// ValidateImmediateCallerAcceptAny implements Runtime.
func (a *runtimeAdapter) ValidateImmediateCallerAcceptAny() {
	a.ctx.ValidateCaller(pattern.Any{})
}

// ValidateImmediateCallerIs implements Runtime.
func (a *runtimeAdapter) ValidateImmediateCallerIs(addrs ...address.Address) {
	a.ctx.ValidateCaller(pattern.AddressIn{Addresses: addrs})
}

// ValidateImmediateCallerType implements Runtime.
func (a *runtimeAdapter) ValidateImmediateCallerType(codes ...cid.Cid) {
	a.ctx.ValidateCaller(pattern.CodeIn{Codes: codes})
}

// CurrentBalance implements Runtime.
func (a *runtimeAdapter) CurrentBalance() abi.TokenAmount {
	return a.ctx.Balance()
}

// ResolveAddress implements Runtime.
func (a *runtimeAdapter) ResolveAddress(addr address.Address) (address.Address, bool) {
	return a.ctx.vm.normalizeAddress(addr)
}

// GetActorCodeCID implements Runtime.
func (a *runtimeAdapter) GetActorCodeCID(addr address.Address) (ret cid.Cid, ok bool) {
	entry, found, err := a.ctx.vm.state.GetActor(a.Context(), addr)
	if !found {
		return cid.Undef, false
	}
	if err != nil {
		panic(err)
	}
	return entry.Code.Cid, true
}

// Abortf implements Runtime.
func (a *runtimeAdapter) Abortf(errExitCode exitcode.ExitCode, msg string, args ...interface{}) {
	runtime.Abortf(errExitCode, msg, args...)
}

// NewActorAddress implements Runtime.
func (a *runtimeAdapter) NewActorAddress() address.Address {
	return a.ctx.NewActorAddress()
}

// CreateActor implements Runtime.
func (a *runtimeAdapter) CreateActor(codeID cid.Cid, addr address.Address) {
	if !builtin.IsBuiltinActor(codeID) {
		runtime.Abortf(exitcode.SysErrorIllegalArgument, "Can only create built-in actors.")
	}

	vmlog.Debugf("creating actor, friendly-name: %s, code: %s, addr: %s\n", builtin.ActorNameByCode(codeID), codeID, addr)

	// Check existing address. If nothing there, create empty actor.
	//
	// Note: we are storing the actors by ActorID *address*
	_, found, err := a.ctx.vm.state.GetActor(a.ctx.vm.context, addr)
	if err != nil {
		panic(err)
	}
	if found {
		runtime.Abortf(exitcode.SysErrorIllegalArgument, "Actor address already exists")
	}

	// Charge gas now that easy checks are done
	a.ctx.gasTank.Charge(gas.PricelistByEpoch(a.ctx.vm.CurrentEpoch()).OnCreateActor(), "CreateActor code %s, address %s", codeID, addr)

	newActor := &types.Actor{
		// make this the right 'type' of actor
		Code:       enccid.NewCid(codeID),
		Balance:    abi.NewTokenAmount(0),
		Head:       enccid.NewCid(EmptyObjectCid),
		CallSeqNum: 0,
	}
	if err := a.ctx.vm.state.SetActor(a.ctx.vm.context, addr, newActor); err != nil {
		panic(err)
	}

	_ = a.ctx.gasTank.TryCharge(gasOnActorExec)
}

// DeleteActor implements Runtime.
func (a *runtimeAdapter) DeleteActor(beneficiary address.Address) {
	a.ctx.DeleteActor(beneficiary)
}

func (a *runtimeAdapter) TotalFilCircSupply() abi.TokenAmount {
	circSupply, err := a.stateView.TotalFilCircSupply(a.CurrEpoch(), a.ctx.vm.state)
	if err != nil {
		runtime.Abortf(exitcode.ErrIllegalState, "failed To get total circ supply: %s", err)
	}
	return circSupply
}

// Context implements Runtime.
// Dragons: this can disappear once we have the storage abstraction
func (a *runtimeAdapter) Context() context.Context {
	return a.ctx.vm.context
}

var nullTraceSpan = func() {}

// StartSpan implements Runtime.
func (a *runtimeAdapter) StartSpan(name string) func() {
	// Dragons: leeave empty for now, add TODO To add this into gfc
	return nullTraceSpan
}

func (a *runtimeAdapter) AbortStateMsg(msg string) {
	runtime.Abortf(101, msg)
}
