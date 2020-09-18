package vmcontext

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/types"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/go-state-types/rt"
	"github.com/filecoin-project/specs-actors/actors/runtime/proof"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/pattern"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
)

type runtimeAdapter struct {
	ctx *invocationContext
}

func (a *runtimeAdapter) Caller() address.Address {
	return a.ctx.Message().Caller()
}

func (a *runtimeAdapter) Receiver() address.Address {
	return a.ctx.Message().Receiver()
}

func (a *runtimeAdapter) ValueReceived() abi.TokenAmount {
	return a.ctx.Message().ValueReceived()
}

func (a *runtimeAdapter) StateCreate(obj cbor.Marshaler) {
	c := a.StorePut(obj)
	a.ctx.rt.state.Commit()
	err := a.(EmptyObjectCid, c)
	if err != nil {
		panic(fmt.Errorf("failed to commit state after creating object: %w", err))
	}
}

func (a *runtimeAdapter) StateReadonly(obj cbor.Unmarshaler) {
	panic("implement me")
}

func (a *runtimeAdapter) StateTransaction(obj cbor.Er, f func()) {
	panic("implement me")
}

func (a *runtimeAdapter) StoreGet(c cid.Cid, o cbor.Unmarshaler) bool {
	panic("implement me")
}

func (a *runtimeAdapter) StorePut(x cbor.Marshaler) cid.Cid {
	panic("implement me")
}

func (a *runtimeAdapter) VerifySignature(signature crypto.Signature, signer address.Address, plaintext []byte) error {
	panic("implement me")
}

func (a *runtimeAdapter) HashBlake2b(data []byte) [32]byte {
	panic("implement me")
}

func (a *runtimeAdapter) ComputeUnsealedSectorCID(reg abi.RegisteredSealProof, pieces []abi.PieceInfo) (cid.Cid, error) {
	panic("implement me")
}

func (a *runtimeAdapter) VerifySeal(vi proof.SealVerifyInfo) error {
	panic("implement me")
}

func (a *runtimeAdapter) BatchVerifySeals(vis map[address.Address][]proof.SealVerifyInfo) (map[address.Address][]bool, error) {
	panic("implement me")
}

func (a *runtimeAdapter) VerifyPoSt(vi proof.WindowPoStVerifyInfo) error {
	panic("implement me")
}

func (a *runtimeAdapter) VerifyConsensusFault(h1, h2, extra []byte) (*specsruntime.ConsensusFault, error) {
	panic("implement me")
}

func (a *runtimeAdapter) NetworkVersion() network.Version {
	panic("implement me")
}

func (a *runtimeAdapter) GetRandomnessFromBeacon(personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) abi.Randomness {
	panic("implement me")
}

func (a *runtimeAdapter) GetRandomnessFromTickets(personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) abi.Randomness {
	panic("implement me")
}

func (a *runtimeAdapter) Send(toAddr address.Address, methodNum abi.MethodNum, params cbor.Marshaler, value abi.TokenAmount, out cbor.Er) exitcode.ExitCode {
	panic("implement me")
}

func (a *runtimeAdapter) ChargeGas(name string, gas int64, virtual int64) {
	panic("implement me")
}

func (a *runtimeAdapter) Log(level rt.LogLevel, msg string, args ...interface{}) {
	panic("implement me")
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
func (a *runtimeAdapter) Send(toAddr address.Address, methodNum abi.MethodNum, params cbg.CBORMarshaler, value abi.TokenAmount) (ret types.SendReturn, errcode exitcode.ExitCode) {
	return a.ctx.Send(toAddr, methodNum, params, value)
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
	a.ctx.CreateActor(codeID, addr)
}

// DeleteActor implements Runtime.
func (a *runtimeAdapter) DeleteActor(beneficiary address.Address) {
	a.ctx.DeleteActor(beneficiary)
}

// SyscallsImpl implements Runtime.
func (a *runtimeAdapter) Syscalls() specsruntime.Syscalls {
	return &syscalls{
		impl:      a.ctx.rt.syscalls,
		ctx:       a.ctx.rt.context,
		gasTank:   a.ctx.gasTank,
		pricelist: a.ctx.rt.pricelist,
		head:      a.ctx.rt.currentHead,
		state:     a.ctx.rt.stateView(),
	}
}

func (a *runtimeAdapter) TotalFilCircSupply() abi.TokenAmount {
	return a.ctx.TotalFilCircSupply()
}

// Context implements Runtime.
// Dragons: this can disappear once we have the storage abstraction
func (a *runtimeAdapter) Context() context.Context {
	return a.ctx.rt.context
}

var  nullTraceSpan = func(){}
// StartSpan implements Runtime.
func (a *runtimeAdapter) StartSpan(name string) func() {
	// Dragons: leeave empty for now, add TODO to add this into gfc
	return nullTraceSpan
}
