package vmcontext

import (
	"context"
	"fmt"
	"math/rand"

	vtypes "github.com/filecoin-project/chain-validation/chain/types"
	vstate "github.com/filecoin-project/chain-validation/state"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-crypto"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	acrypto "github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	gfcrypto "github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	gfbuiltin "github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gascost"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/interpreter"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storage"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vmsupport"
)

var _ vstate.Factories = &Factories{}
var _ vstate.VMWrapper = (*ValidationVMWrapper)(nil)
var _ vstate.Applier = (*ValidationApplier)(nil)
var _ vstate.KeyManager = (*KeyManager)(nil)

type Factories struct {
	vstate.Applier
}

func NewFactories() *Factories {
	factory := &Factories{&ValidationApplier{}}
	return factory
}

func (f *Factories) NewState() vstate.VMWrapper {
	return NewState()
}

func (f *Factories) NewKeyManager() vstate.KeyManager {
	return newKeyManager()
}

type fakeRandSrc struct {
}

func (r fakeRandSrc) Randomness(_ context.Context, _ acrypto.DomainSeparationTag, _ abi.ChainEpoch, _ []byte) (abi.Randomness, error) {
	panic("implement me")
}

func (f *Factories) NewRandomnessSource() vstate.RandomnessSource {
	return &fakeRandSrc{}
}

func (f *Factories) NewValidationConfig() vstate.ValidationConfig {
	return &ValidationConfig{
		// TODO enable this when ready https://github.com/filecoin-project/go-filecoin/issues/3801
		trackGas:         false,
		checkExitCode:    true,
		checkReturnValue: true,
	}
}

//
// ValidationConfig
//

type ValidationConfig struct {
	trackGas         bool
	checkExitCode    bool
	checkReturnValue bool
}

func (v ValidationConfig) ValidateGas() bool {
	return v.trackGas
}

func (v ValidationConfig) ValidateExitCode() bool {
	return v.checkExitCode
}

func (v ValidationConfig) ValidateReturnValue() bool {
	return v.checkReturnValue
}

//
// VMWrapper
//

func NewState() *ValidationVMWrapper {
	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	cst := cborutil.NewIpldStore(bs)
	vmstrg := storage.NewStorage(bs)
	vm := NewVM(gfbuiltin.DefaultActors, &vmstrg, state.NewState(cst), &vmsupport.FakeSyscalls{})
	return &ValidationVMWrapper{
		vm: &vm,
	}
}

type ValidationVMWrapper struct {
	vm *VM
}

// Root implements ValidationVMWrapper.
func (w *ValidationVMWrapper) Root() cid.Cid {
	root, dirty := w.vm.state.Root()
	if !dirty {
		return root
	}

	root, err := w.vm.state.Commit(w.vm.context)
	if err != nil {
		panic(err)
	}
	return root
}

// Store implements ValidationVMWrapper.
func (w *ValidationVMWrapper) Store() adt.Store {
	return w.vm.ContextStore()
}

// Actor implements ValidationVMWrapper.
func (w *ValidationVMWrapper) Actor(addr address.Address) (vstate.Actor, error) {
	idAddr, found := w.vm.normalizeAddress(addr)
	if !found {
		return nil, fmt.Errorf("failed to normalize address: %s", addr)
	}

	a, found, err := w.vm.state.GetActor(w.vm.context, idAddr)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("actor not found")
	}
	return &actorWrapper{a}, nil
}

// CreateActor implements ValidationVMWrapper.
func (w *ValidationVMWrapper) CreateActor(code cid.Cid, addr address.Address, balance abi.TokenAmount, newState runtime.CBORMarshaler) (vstate.Actor, address.Address, error) {
	idAddr := addr
	if addr.Protocol() != address.ID {
		// go through init to register
		initActorEntry, found, err := w.vm.state.GetActor(w.vm.context, builtin.InitActorAddr)
		if err != nil {
			return nil, address.Undef, err
		}
		if !found {
			return nil, address.Undef, fmt.Errorf("actor not found")
		}

		// get a view into the actor state
		var initState init_.State
		if _, err := w.vm.store.Get(w.vm.context, initActorEntry.Head.Cid, &initState); err != nil {
			return nil, address.Undef, err
		}

		// add addr to inits map
		idAddr, err = initState.MapAddressToNewID(w.vm.ContextStore(), addr)
		if err != nil {
			return nil, address.Undef, err
		}

		// persist the init actor state
		initHead, _, err := w.vm.store.Put(w.vm.context, &initState)
		if err != nil {
			return nil, address.Undef, err
		}
		initActorEntry.Head = enccid.NewCid(initHead)
		if err := w.vm.state.SetActor(w.vm.context, builtin.InitActorAddr, initActorEntry); err != nil {
			return nil, address.Undef, err
		}
		// persist state below
	}

	// create actor on state stree

	// store newState
	head, _, err := w.vm.store.Put(w.vm.context, newState)
	if err != nil {
		return nil, address.Undef, err
	}

	// create and store actor object
	a := &actor.Actor{
		Code:    enccid.NewCid(code),
		Head:    enccid.NewCid(head),
		Balance: balance,
	}
	if err := w.vm.state.SetActor(w.vm.context, idAddr, a); err != nil {
		return nil, address.Undef, err
	}

	if err := w.PersistChanges(); err != nil {
		return nil, address.Undef, err
	}

	return &actorWrapper{a}, idAddr, nil
}

// SetActorState implements ValidationVMWrapper.
func (w *ValidationVMWrapper) SetActorState(addr address.Address, balance big.Int, state runtime.CBORMarshaler) (vstate.Actor, error) {
	idAddr, ok := w.vm.normalizeAddress(addr)
	if !ok {
		return nil, fmt.Errorf("actor not found")
	}

	a, found, err := w.vm.state.GetActor(w.vm.context, idAddr)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("actor not found")
	}
	// store state
	head, _, err := w.vm.store.Put(w.vm.context, state)
	if err != nil {
		return nil, err
	}
	// update fields
	a.Head = enccid.NewCid(head)
	a.Balance = balance

	if err := w.vm.state.SetActor(w.vm.context, idAddr, a); err != nil {
		return nil, err
	}

	if err := w.PersistChanges(); err != nil {
		return nil, err
	}

	return &actorWrapper{a}, nil
}

func (w *ValidationVMWrapper) PersistChanges() error {
	if _, err := w.vm.commit(); err != nil {
		return err
	}
	return nil
}

//
// Applier
//

type ValidationApplier struct{}

func (a *ValidationApplier) ApplyMessage(context *vtypes.ExecutionContext, state vstate.VMWrapper, msg *vtypes.Message) (vtypes.MessageReceipt, error) {
	st := state.(*ValidationVMWrapper)

	// set epoch
	// Note: this would have normally happened during `ApplyTipset()`
	st.vm.currentEpoch = context.Epoch
	st.vm.pricelist = gascost.PricelistByEpoch(context.Epoch)

	// map message
	// Dragons: fix after cleaning up our msg
	ourmsg := &types.UnsignedMessage{
		To:         msg.To,
		From:       msg.From,
		CallSeqNum: uint64(msg.CallSeqNum),
		Value:      msg.Value,
		Method:     msg.Method,
		Params:     msg.Params,
		GasPrice:   msg.GasPrice,
		GasLimit:   types.GasUnits(msg.GasLimit),
	}

	// invoke vm
	ourreceipt, _, _ := st.vm.applyMessage(ourmsg, ourmsg.OnChainLen(), &fakeRandSrc{})

	// commit and persist changes
	// Note: this is not done on production for each msg
	if err := st.PersistChanges(); err != nil {
		return vtypes.MessageReceipt{}, err
	}

	// map receipt
	receipt := vtypes.MessageReceipt{
		ExitCode:    ourreceipt.ExitCode,
		ReturnValue: ourreceipt.ReturnValue,
		GasUsed:     ourreceipt.GasUsed.AsBigInt(),
	}

	return receipt, nil
}

func toOurBlockMessageInfoType(theirs []vtypes.BlockMessagesInfo) []interpreter.BlockMessagesInfo {
	ours := make([]interpreter.BlockMessagesInfo, len(theirs))
	for i, bm := range theirs {
		ours[i].Miner = bm.Miner
		ours[i].TicketCount = bm.TicketCount
		for _, blsMsg := range bm.BLSMessages {
			ourbls := &types.UnsignedMessage{
				To:         blsMsg.To,
				From:       blsMsg.From,
				CallSeqNum: uint64(blsMsg.CallSeqNum),
				Value:      blsMsg.Value,
				Method:     blsMsg.Method,
				Params:     blsMsg.Params,
				GasPrice:   blsMsg.GasPrice,
				GasLimit:   types.GasUnits(blsMsg.GasLimit),
			}
			ours[i].BLSMessages = append(ours[i].BLSMessages, ourbls)
		}
		for _, secpMsg := range bm.SECPMessages {
			oursecp := &types.SignedMessage{
				Message: types.UnsignedMessage{
					To:         secpMsg.Message.To,
					From:       secpMsg.Message.From,
					CallSeqNum: uint64(secpMsg.Message.CallSeqNum),
					Value:      secpMsg.Message.Value,
					Method:     secpMsg.Message.Method,
					Params:     secpMsg.Message.Params,
					GasPrice:   secpMsg.Message.GasPrice,
					GasLimit:   types.GasUnits(secpMsg.Message.GasLimit),
				},
				Signature: secpMsg.Signature,
			}
			ours[i].SECPMessages = append(ours[i].SECPMessages, oursecp)
		}
	}
	return ours
}

func (a *ValidationApplier) ApplyTipSetMessages(state vstate.VMWrapper, blocks []vtypes.BlockMessagesInfo, epoch abi.ChainEpoch, rnd vstate.RandomnessSource) ([]vtypes.MessageReceipt, error) {
	st := state.(*ValidationVMWrapper)

	ourBlkMsgs := toOurBlockMessageInfoType(blocks)
	receipts, err := st.vm.ApplyTipSetMessages(ourBlkMsgs, epoch, rnd)
	if err != nil {
		return nil, err
	}

	theirReceipts := make([]vtypes.MessageReceipt, len(receipts))
	for i, r := range receipts {
		theirReceipts[i] = vtypes.MessageReceipt{
			ExitCode:    r.ExitCode,
			ReturnValue: r.ReturnValue,
			GasUsed:     r.GasUsed.AsBigInt(),
		}
	}

	return theirReceipts, nil
}

//
// KeyManager
//

type KeyManager struct {
	// Private keys by address
	keys map[address.Address]*gfcrypto.KeyInfo

	// Seed for deterministic secp key generation.
	secpSeed int64
	// Seed for deterministic bls key generation.
	blsSeed int64 // nolint: structcheck
}

func newKeyManager() *KeyManager {
	return &KeyManager{
		keys:     make(map[address.Address]*gfcrypto.KeyInfo),
		secpSeed: 0,
	}
}

func (k *KeyManager) NewSECP256k1AccountAddress() address.Address {
	secpKey := k.newSecp256k1Key()
	addr, err := secpKey.Address()
	if err != nil {
		panic(err)
	}
	k.keys[addr] = secpKey
	return addr
}

func (k *KeyManager) NewBLSAccountAddress() address.Address {
	blsKey := k.newBLSKey()
	addr, err := blsKey.Address()
	if err != nil {
		panic(err)
	}
	k.keys[addr] = blsKey
	return addr
}

func (k *KeyManager) Sign(addr address.Address, data []byte) (acrypto.Signature, error) {
	ki, ok := k.keys[addr]
	if !ok {
		return acrypto.Signature{}, fmt.Errorf("unknown address %v", addr)
	}
	return gfcrypto.Sign(data, ki.PrivateKey, ki.SigType)
}

func (k *KeyManager) newSecp256k1Key() *gfcrypto.KeyInfo {
	randSrc := rand.New(rand.NewSource(k.secpSeed))
	prv, err := crypto.GenerateKeyFromSeed(randSrc)
	if err != nil {
		panic(err)
	}
	k.secpSeed++
	return &gfcrypto.KeyInfo{
		SigType:    acrypto.SigTypeSecp256k1,
		PrivateKey: prv,
	}
}

func (k *KeyManager) newBLSKey() *gfcrypto.KeyInfo {
	// FIXME: bls needs deterministic key generation
	//sk := ffi.PrivateKeyGenerate(s.blsSeed)
	// s.blsSeed++
	sk := ffi.PrivateKeyGenerate()
	return &gfcrypto.KeyInfo{
		SigType:    acrypto.SigTypeBLS,
		PrivateKey: sk[:],
	}
}

//
// Actor
//

type actorWrapper struct {
	*actor.Actor
}

func (a *actorWrapper) Code() cid.Cid {
	return a.Actor.Code.Cid
}
func (a *actorWrapper) Head() cid.Cid {
	return a.Actor.Head.Cid
}
func (a *actorWrapper) CallSeqNum() int64 {
	return int64(a.Actor.CallSeqNum)
}
func (a *actorWrapper) Balance() abi.TokenAmount {
	return a.Actor.Balance
}
