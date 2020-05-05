package vmcontext

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-crypto"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/account"
	"github.com/filecoin-project/specs-actors/actors/builtin/cron"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/builtin/multisig"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	"github.com/filecoin-project/specs-actors/actors/builtin/reward"
	"github.com/filecoin-project/specs-actors/actors/builtin/system"
	acrypto "github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/filecoin-project/specs-actors/actors/puppet"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/util/adt"

	vtypes "github.com/filecoin-project/chain-validation/chain/types"
	vdriver "github.com/filecoin-project/chain-validation/drivers"
	vstate "github.com/filecoin-project/chain-validation/state"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	gfcrypto "github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gascost"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/interpreter"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storage"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

var _ vstate.Factories = &Factories{}
var _ vstate.VMWrapper = (*ValidationVMWrapper)(nil)
var _ vstate.Applier = (*ValidationApplier)(nil)
var _ vstate.KeyManager = (*KeyManager)(nil)

type Factories struct {
	config *ValidationConfig
}

func NewFactories(config *ValidationConfig) *Factories {
	factory := &Factories{config}
	return factory
}

func (f *Factories) NewStateAndApplier() (vstate.VMWrapper, vstate.Applier) {
	st := NewState()
	return st, &ValidationApplier{state: st}
}

func (f *Factories) NewKeyManager() vstate.KeyManager {
	return newKeyManager()
}

type fakeRandSrc struct {
}

func (r fakeRandSrc) Randomness(_ context.Context, _ acrypto.DomainSeparationTag, _ abi.ChainEpoch, _ []byte) (abi.Randomness, error) {
	return abi.Randomness("sausages"), nil
}

func (f *Factories) NewRandomnessSource() vstate.RandomnessSource {
	return &fakeRandSrc{}
}

func (f *Factories) NewValidationConfig() vstate.ValidationConfig {
	return f.config
}

//
// ValidationConfig
//

type ValidationConfig struct {
	trackGas         bool
	checkExitCode    bool
	checkReturnValue bool
	checkStateRoot   bool
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

func (v ValidationConfig) ValidateStateRoot() bool {
	return v.checkStateRoot
}

//
// VMWrapper
//

type specialSyscallWrapper struct {
	internal *vdriver.ChainValidationSyscalls
}

func (s specialSyscallWrapper) VerifySignature(_ context.Context, _ SyscallsStateView, signature gfcrypto.Signature, signer address.Address, plaintext []byte) error {
	return s.internal.VerifySignature(signature, signer, plaintext)
}

func (s specialSyscallWrapper) HashBlake2b(data []byte) [32]byte {
	return s.internal.HashBlake2b(data)
}

func (s specialSyscallWrapper) ComputeUnsealedSectorCID(_ context.Context, proof abi.RegisteredProof, pieces []abi.PieceInfo) (cid.Cid, error) {
	return s.internal.ComputeUnsealedSectorCID(proof, pieces)
}

func (s specialSyscallWrapper) VerifySeal(_ context.Context, info abi.SealVerifyInfo) error {
	return s.internal.VerifySeal(info)
}

func (s specialSyscallWrapper) VerifyPoSt(_ context.Context, info abi.WindowPoStVerifyInfo) error {
	return s.internal.VerifyPoSt(info)
}

func (s specialSyscallWrapper) VerifyConsensusFault(_ context.Context, h1, h2, extra []byte, _ block.TipSetKey, _ SyscallsStateView) (*runtime.ConsensusFault, error) {
	return s.internal.VerifyConsensusFault(h1, h2, extra)
}

var ChainvalActors = dispatch.NewBuilder().
	Add(builtin.InitActorCodeID, &init_.Actor{}).
	Add(builtin.AccountActorCodeID, &account.Actor{}).
	Add(builtin.MultisigActorCodeID, &multisig.Actor{}).
	Add(builtin.PaymentChannelActorCodeID, &paych.Actor{}).
	Add(builtin.StoragePowerActorCodeID, &power.Actor{}).
	Add(builtin.StorageMarketActorCodeID, &market.Actor{}).
	Add(builtin.StorageMinerActorCodeID, &miner.Actor{}).
	Add(builtin.SystemActorCodeID, &system.Actor{}).
	Add(builtin.RewardActorCodeID, &reward.Actor{}).
	Add(builtin.CronActorCodeID, &cron.Actor{}).
	// add the puppet actor
	Add(puppet.PuppetActorCodeID, &puppet.Actor{}).
	Build()

func NewState() *ValidationVMWrapper {
	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	cst := cborutil.NewIpldStore(bs)
	vmstrg := storage.NewStorage(bs)

	vm := NewVM(ChainvalActors, &vmstrg, state.NewState(cst), specialSyscallWrapper{vdriver.NewChainValidationSyscalls()})
	return &ValidationVMWrapper{
		vm: &vm,
	}
}

type ValidationVMWrapper struct {
	vm *VM
}

func (w *ValidationVMWrapper) NewVM() {
	return
}

// Root implements ValidationVMWrapper.
func (w *ValidationVMWrapper) Root() cid.Cid {
	root, dirty := w.vm.state.Root()
	if dirty {
		panic("vm state is dirty")
	}
	return root
}

// Get the value at key from vm store
func (w *ValidationVMWrapper) StoreGet(key cid.Cid, out runtime.CBORUnmarshaler) error {
	return w.vm.ContextStore().Get(context.Background(), key, out)
}

// Put `value` into vm store
func (w *ValidationVMWrapper) StorePut(value runtime.CBORMarshaler) (cid.Cid, error) {
	return w.vm.ContextStore().Put(context.Background(), value)
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

type ValidationApplier struct {
	state *ValidationVMWrapper
}

func (a *ValidationApplier) ApplyMessage(epoch abi.ChainEpoch, msg *vtypes.Message) (vtypes.ApplyMessageResult, error) {
	// Prepare message and VM.
	ourMsg := a.preApplyMessage(epoch, msg)

	// Invoke.
	ourreceipt, penalty, reward := a.state.vm.applyMessage(ourMsg, ourMsg.OnChainLen(), &fakeRandSrc{})

	// Persist changes.
	receipt, err := a.postApplyMessage(ourreceipt)
	return vtypes.ApplyMessageResult{
		Receipt: receipt,
		Penalty: penalty,
		Reward:  reward,
		Root:    a.state.Root().String(),
	}, err
}

func (a *ValidationApplier) ApplySignedMessage(epoch abi.ChainEpoch, msg *vtypes.SignedMessage) (vtypes.ApplyMessageResult, error) {

	// Prepare message and VM.
	ourMsg := a.preApplyMessage(epoch, &msg.Message)
	ourSigned := &types.SignedMessage{
		Message:   *ourMsg,
		Signature: msg.Signature,
	}

	// Invoke.
	ourreceipt, penalty, reward := a.state.vm.applyMessage(ourMsg, ourSigned.OnChainLen(), &fakeRandSrc{})

	// Persist changes.
	receipt, err := a.postApplyMessage(ourreceipt)
	return vtypes.ApplyMessageResult{
		Receipt: receipt,
		Penalty: penalty,
		Reward:  reward,
		Root:    a.state.Root().String(),
	}, err
}

func (a *ValidationApplier) preApplyMessage(epoch abi.ChainEpoch, msg *vtypes.Message) *types.UnsignedMessage {
	// set epoch
	// Note: this would have normally happened during `ApplyTipset()`
	a.state.vm.currentEpoch = epoch
	a.state.vm.pricelist = gascost.PricelistByEpoch(epoch)

	// map message
	return toOurMessage(msg)
}

func (a *ValidationApplier) postApplyMessage(ourreceipt message.Receipt) (vtypes.MessageReceipt, error) {
	// commit and persist changes
	// Note: this is not done on production for each msg
	if err := a.state.PersistChanges(); err != nil {
		return vtypes.MessageReceipt{}, err
	}

	// map receipt
	return vtypes.MessageReceipt{
		ExitCode:    ourreceipt.ExitCode,
		ReturnValue: ourreceipt.ReturnValue,
		GasUsed:     vtypes.GasUnits(ourreceipt.GasUsed),
	}, nil
}

func toOurMessage(theirs *vtypes.Message) *types.UnsignedMessage {
	return &types.UnsignedMessage{
		To:         theirs.To,
		From:       theirs.From,
		CallSeqNum: theirs.CallSeqNum,
		Value:      theirs.Value,
		Method:     theirs.Method,
		Params:     theirs.Params,
		GasPrice:   theirs.GasPrice,
		GasLimit:   gas.Unit(theirs.GasLimit),
	}

}

func toOurBlockMessageInfoType(theirs []vtypes.BlockMessagesInfo) []interpreter.BlockMessagesInfo {
	ours := make([]interpreter.BlockMessagesInfo, len(theirs))
	for i, bm := range theirs {
		ours[i].Miner = bm.Miner
		for _, blsMsg := range bm.BLSMessages {
			ourbls := &types.UnsignedMessage{
				To:         blsMsg.To,
				From:       blsMsg.From,
				CallSeqNum: blsMsg.CallSeqNum,
				Value:      blsMsg.Value,
				Method:     blsMsg.Method,
				Params:     blsMsg.Params,
				GasPrice:   blsMsg.GasPrice,
				GasLimit:   gas.Unit(blsMsg.GasLimit),
			}
			ours[i].BLSMessages = append(ours[i].BLSMessages, ourbls)
		}
		for _, secpMsg := range bm.SECPMessages {
			oursecp := &types.SignedMessage{
				Message: types.UnsignedMessage{
					To:         secpMsg.Message.To,
					From:       secpMsg.Message.From,
					CallSeqNum: secpMsg.Message.CallSeqNum,
					Value:      secpMsg.Message.Value,
					Method:     secpMsg.Message.Method,
					Params:     secpMsg.Message.Params,
					GasPrice:   secpMsg.Message.GasPrice,
					GasLimit:   gas.Unit(secpMsg.Message.GasLimit),
				},
				Signature: secpMsg.Signature,
			}
			ours[i].SECPMessages = append(ours[i].SECPMessages, oursecp)
		}
	}
	return ours
}

func (a *ValidationApplier) ApplyTipSetMessages(epoch abi.ChainEpoch, blocks []vtypes.BlockMessagesInfo, rnd vstate.RandomnessSource) (vtypes.ApplyTipSetResult, error) {

	ourBlkMsgs := toOurBlockMessageInfoType(blocks)
	// TODO: pass through parameter when chain validation type signature is updated to propagate it
	head := block.NewTipSetKey()
	receipts, err := a.state.vm.ApplyTipSetMessages(ourBlkMsgs, head, epoch, rnd)
	if err != nil {
		return vtypes.ApplyTipSetResult{}, err
	}

	theirReceipts := make([]vtypes.MessageReceipt, len(receipts))
	for i, r := range receipts {
		theirReceipts[i] = vtypes.MessageReceipt{
			ExitCode:    r.ExitCode,
			ReturnValue: r.ReturnValue,
			GasUsed:     vtypes.GasUnits(r.GasUsed),
		}
	}

	return vtypes.ApplyTipSetResult{
		Receipts: theirReceipts,
		Root:     a.state.Root().String(),
	}, nil
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
	sk := [32]byte{}
	sk[0] = uint8(k.blsSeed) // hack to keep gas values and state roots determinist
	k.blsSeed++
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
func (a *actorWrapper) CallSeqNum() uint64 {
	return a.Actor.CallSeqNum
}
func (a *actorWrapper) Balance() abi.TokenAmount {
	return a.Actor.Balance
}
