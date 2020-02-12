package vmcontext

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/pattern"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"
)

type runtimeAdapter struct {
	ctx invocationContext
}

var _ specsruntime.Runtime = (*runtimeAdapter)(nil)

// Message implements Runtime.
func (a *runtimeAdapter) Message() specsruntime.Message {
	return a.ctx.Message()
}

// NetworkName implements Runtime.
func (a *runtimeAdapter) NetworkName() string {
	panic("Will get nuked")
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
func (a *runtimeAdapter) GetRandomness(epoch abi.ChainEpoch) abi.RandomnessSeed {
	// Dragons: type missmatch
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
func (a *runtimeAdapter) Send(toAddr address.Address, methodNum abi.MethodNum, params specsruntime.CBORMarshaler, value abi.TokenAmount) (specsruntime.SendReturn, exitcode.ExitCode) {
	// Dragons: needs to catch panic and flatten it out
	// Dragons: PR for use unmarshable
	panic("TODO")
}

// Abortf implements Runtime.
func (a *runtimeAdapter) Abortf(errExitCode exitcode.ExitCode, msg string, args ...interface{}) {
	runtime.Abortf(errExitCode, msg, args...)
}

// NewActorAddress implements Runtime.
func (a *runtimeAdapter) NewActorAddress() address.Address {
	// Dragons: Could potentially be removed
	panic("TODO")
}

// CreateActor implements Runtime.
func (a *runtimeAdapter) CreateActor(codeID cid.Cid, address address.Address) {
	// Dragons: contract missmatch
	a.ctx.CreateActor(0, codeID, nil)
}

// DeleteActor implements Runtime.
func (a *runtimeAdapter) DeleteActor() {
	panic("TODO")
}

// Syscalls implements Runtime.
func (a *runtimeAdapter) Syscalls() specsruntime.Syscalls {
	// Dragons: add `Syscalls()` to our Runtime.
	panic("TODO")
}

// Dragons: this can dissapear once we have the storage abstraction

// Context implements Runtime.
func (a *runtimeAdapter) Context() context.Context {
	panic("TODO")
}

// StartSpan implements Runtime.
func (a *runtimeAdapter) StartSpan(name string) specsruntime.TraceSpan {
	// Review: why here? and why in this form?
	// Dragons: leeave empty for now, add TODO to add this into gfc
	panic("TODO")
}

// Dragons: have the VM take a SysCalls object on construction (it will need a wrapper to charge gas)
type syscallsWrapper struct {
	ctx runtime.ExtendedInvocationContext
}

var _ specsruntime.Syscalls = (*syscallsWrapper)(nil)

// VerifySignature implements Syscalls.
func (w syscallsWrapper) VerifySignature(signature crypto.Signature, signer address.Address, plaintext []byte) bool {
	panic("TODO")
}

// Hash_SHA256 implements Syscalls.
func (w syscallsWrapper) Hash_SHA256(data []byte) []byte {
	// Review: why the underscore?
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
