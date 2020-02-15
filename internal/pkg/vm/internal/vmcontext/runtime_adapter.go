package vmcontext

import (
	"context"
	"runtime/debug"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"

	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/pattern"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
)

type runtimeAdapter struct {
	ctx invocationContext
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

// GetActorCodeCID implements Runtime.
func (a *runtimeAdapter) GetActorCodeCID(addr address.Address) (ret cid.Cid, ok bool) {
	entry, err := a.ctx.rt.state.GetActor(context.Background(), addr)
	if err != nil {
		return cid.Undef, false
	}
	return entry.Code.Cid, true
}

// GetRandomness implements Runtime.
func (a *runtimeAdapter) GetRandomness(tag crypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) abi.Randomness {
	return a.ctx.Runtime().Randomness(epoch)
}

// State implements Runtime.
func (a *runtimeAdapter) State() specsruntime.StateHandle {
	// Dragons: interface missmatch, takes cborers, we need to wait for the new actor code to land and remove the old
	// return a.ctx.State()
	return nil
}

// Store implements Runtime.
func (a *runtimeAdapter) Store() specsruntime.Store {
	// Dragons: interface missmatch, takes cborers, we need to wait for the new actor code to land and remove the old
	// return a.ctx.Runtime().Storage()
	return nil
}

// Send implements Runtime.
func (a *runtimeAdapter) Send(toAddr address.Address, methodNum abi.MethodNum, params specsruntime.CBORMarshaler, value abi.TokenAmount) (ret specsruntime.SendReturn, errcode exitcode.ExitCode) {
	// Dragons: move impl to actual send once we delete the old actors and can change the signature
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case runtime.ExecutionPanic:
				p := r.(runtime.ExecutionPanic)
				// TODO: log
				ret = nil
				errcode = p.Code()
				return
			default:
				// do not trap unknown panics
				debug.PrintStack()
				panic(r)
			}
		}
	}()

	panic("update actors first")
	// return a.ctx.Send(to, methodNum, value, params)
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
	// Dragons: replace the method in invocation context once the new actors land
	// Dragons: there were some changes in spec, revise
	if !isBuiltinActor(codeID) {
		runtime.Abortf(exitcode.ErrIllegalArgument, "Can only create built-in actors.")
	}

	if builtin.IsSingletonActor(codeID) {
		runtime.Abortf(exitcode.ErrIllegalArgument, "Can only have one instance of singleton actors.")
	}

	// Check existing address. If nothing there, create empty actor.
	//
	// Note: we are storing the actors by ActorID *address*
	newActor, _, err := a.ctx.rt.state.GetOrCreateActor(context.TODO(), addr, func() (*actor.Actor, address.Address, error) {
		return &actor.Actor{}, addr, nil
	})

	if err != nil {
		panic(err)
	}

	if !newActor.Empty() {
		runtime.Abortf(exitcode.ErrIllegalArgument, "Actor address already exists")
	}

	newActor.Balance = abi.NewTokenAmount(0)
	// make this the right 'type' of actor
	newActor.Code = e.NewCid(codeID)
}

// DeleteActor implements Runtime.
func (a *runtimeAdapter) DeleteActor() {
	if err := a.ctx.rt.state.DeleteActor(a.Context(), a.ctx.msg.to); err != nil {
		panic(err)
	}
}

// Syscalls implements Runtime.
func (a *runtimeAdapter) Syscalls() specsruntime.Syscalls {
	// Dragons: add `Syscalls()` to our Runtime.
	panic("TODO")
}

// Context implements Runtime.
func (a *runtimeAdapter) Context() context.Context {
	// Dragons: this can disappear once we have the storage abstraction
	return a.ctx.rt.context
}

type nullTraceSpan struct{}

func (*nullTraceSpan) End() {}

// StartSpan implements Runtime.
func (a *runtimeAdapter) StartSpan(name string) specsruntime.TraceSpan {
	// Dragons: leeave empty for now, add TODO to add this into gfc
	return &nullTraceSpan{}
}

// Dragons: have the VM take a SysCalls object on construction (it will need a wrapper to charge gas)
type syscallsWrapper struct {
}

var _ specsruntime.Syscalls = (*syscallsWrapper)(nil)

// VerifySignature implements Syscalls.
func (w syscallsWrapper) VerifySignature(signature crypto.Signature, signer address.Address, plaintext []byte) bool {
	panic("TODO")
}

// HashBlake2b implements Syscalls.
func (w syscallsWrapper) HashBlake2b(data []byte) [8]byte { // nolint: golint
	panic("TODO")
}

// ComputeUnsealedSectorCID implements Syscalls.
func (w syscallsWrapper) ComputeUnsealedSectorCID(sectorSize abi.SectorSize, pieces []abi.PieceInfo) (cid.Cid, error) {
	panic("TODO")
}

// VerifySeal implements Syscalls.
func (w syscallsWrapper) VerifySeal(sectorSize abi.SectorSize, vi abi.SealVerifyInfo) bool {
	panic("TODO")
}

// VerifyPoSt implements Syscalls.
func (w syscallsWrapper) VerifyPoSt(sectorSize abi.SectorSize, vi abi.PoStVerifyInfo) bool {
	panic("TODO")
}

// VerifyConsensusFault implements Syscalls.
func (w syscallsWrapper) VerifyConsensusFault(h1, h2 []byte) bool {
	panic("TODO")
}
