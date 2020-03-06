package vmcontext

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/pattern"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
)

type runtimeAdapter struct {
	ctx *invocationContext
}

var _ specsruntime.Runtime = (*runtimeAdapter)(nil)

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
	return a.ctx.rt.normalizeAddress(addr)
}

// GetActorCodeCID implements Runtime.
func (a *runtimeAdapter) GetActorCodeCID(addr address.Address) (ret cid.Cid, ok bool) {
	entry, found, err := a.ctx.rt.state.GetActor(context.Background(), addr)
	if !found {
		return cid.Undef, false
	}
	if err != nil {
		panic(err)
	}
	return entry.Code.Cid, true
}

// GetRandomness implements Runtime.
func (a *runtimeAdapter) GetRandomness(tag crypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) abi.Randomness {
	randomness, err := a.ctx.randSource.Randomness(a.Context(), tag, epoch, entropy)
	if err != nil {
		panic(err)
	}
	return randomness
}

// State implements Runtime.
func (a *runtimeAdapter) State() specsruntime.StateHandle {
	return a.ctx.State()
}

// Store implements Runtime.
func (a *runtimeAdapter) Store() specsruntime.Store {
	return a.ctx.Store()
}

// Send implements Runtime.
func (a *runtimeAdapter) Send(toAddr address.Address, methodNum abi.MethodNum, params specsruntime.CBORMarshaler, value abi.TokenAmount) (ret specsruntime.SendReturn, errcode exitcode.ExitCode) {
	return a.ctx.Send(toAddr, methodNum, params, value)
}

// Abortf implements Runtime.
func (a *runtimeAdapter) Abortf(errExitCode exitcode.ExitCode, msg string, args ...interface{}) {
	runtime.Abortf(errExitCode, msg, args...)
}

// NewActorAddress implements Runtime.
func (a *runtimeAdapter) NewActorAddress() address.Address {
	actorAddr, err := computeActorAddress(a.ctx.msg.from, a.ctx.msg.callSeqNumber)
	if err != nil {
		panic("Could not create address for actor")
	}
	return actorAddr
}

// CreateActor implements Runtime.
func (a *runtimeAdapter) CreateActor(codeID cid.Cid, addr address.Address) {
	a.ctx.CreateActor(codeID, addr)
}

// DeleteActor implements Runtime.
func (a *runtimeAdapter) DeleteActor() {
	a.ctx.DeleteActor()
}

// SyscallsImpl implements Runtime.
func (a *runtimeAdapter) Syscalls() specsruntime.Syscalls {
	return &syscalls{
		impl:        a.ctx.rt.syscalls,
		ctx:         a.ctx.rt.context,
		gasTank:     a.ctx.gasTank,
		pricelist:   a.ctx.rt.pricelist,
		sigResolver: a.ctx.rt,
	}
}

// Context implements Runtime.
// Dragons: this can disappear once we have the storage abstraction
func (a *runtimeAdapter) Context() context.Context {
	return a.ctx.rt.context
}

type nullTraceSpan struct{}

func (*nullTraceSpan) End() {}

// StartSpan implements Runtime.
func (a *runtimeAdapter) StartSpan(name string) specsruntime.TraceSpan {
	// Dragons: leeave empty for now, add TODO to add this into gfc
	return &nullTraceSpan{}
}
