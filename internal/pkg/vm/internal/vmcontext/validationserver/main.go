package main

import (
	"net/http"

	"github.com/filecoin-project/chain-validation/chain/types"
	"github.com/filecoin-project/chain-validation/drivers"
	"github.com/filecoin-project/chain-validation/state"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	rpc "github.com/gorilla/rpc/v2"
	jsonrpc "github.com/gorilla/rpc/v2/json"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/vmcontext"
)

var log = logging.Logger("go-filecoin")

func init() {
	logging.SetAllLoggers(logging.LevelInfo)
}

type ConfigReply struct {
	TrackGas         bool `json:"trackGas"`
	CheckExitCode    bool `json:"checkExitCode"`
	CheckReturnValue bool `json:"checkReturnValue"`
	CheckStateRoot   bool `json:"checkStateRoot"`

	TestSuite []string `json:"testSuite"`
}

type ConfigService struct{}

func (c *ConfigService) Config(r *http.Request, args *struct{}, reply *ConfigReply) error {
	reply.CheckExitCode = true
	reply.CheckReturnValue = true
	reply.CheckStateRoot = true
	reply.TrackGas = false
	reply.TestSuite = []string{"TestOne", "TestTwo", "TestThree"}

	log.Infow("ConfigService.Config", "reply", reply)
	return nil
}

type ApplyMessageArgs struct {
	Epoch   abi.ChainEpoch
	Message *types.Message
}

type ApplySignedMessageArgs struct {
	Epoch         abi.ChainEpoch
	SignedMessage *types.SignedMessage
}

type ApplyMessageReply struct {
	Receipt types.MessageReceipt
	Penalty abi.TokenAmount
	Reward  abi.TokenAmount
	Root    cid.Cid
}

type ApplyTipSetMessagesArgs struct {
	Epoch       abi.ChainEpoch
	ParentState cid.Cid
	Blocks      []types.BlockMessagesInfo
	Randomness  abi.Randomness
}

type ApplyTipSetMessagesReply struct {
	Receipts []types.MessageReceipt
	Root     cid.Cid
}

func NewApplierService(applier state.Applier, sw state.VMWrapper) *ApplierService {
	return &ApplierService{applier: applier}
}

type ApplierService struct {
	applier state.Applier
	sw      state.VMWrapper
}

func (a *VmWrapperService) ApplyMessage(r *http.Request, args *ApplyMessageArgs, reply *ApplyMessageReply) error {
	result, err := a.applier.ApplyMessage(args.Epoch, args.Message)
	if err != nil {
		return err
	}
	reply.Receipt = result.Receipt
	reply.Reward = result.Reward
	reply.Penalty = result.Penalty
	reply.Root = a.vm.Root()
	log.Infow("ApplierService.ApplyMessage", "args", args, "reply", reply)
	return nil
}

func (a *VmWrapperService) ApplySignedMessage(r *http.Request, args *ApplySignedMessageArgs, reply *ApplyMessageReply) error {
	result, err := a.applier.ApplySignedMessage(args.Epoch, args.SignedMessage)
	if err != nil {
		return err
	}
	reply.Reward = result.Reward
	reply.Penalty = result.Penalty
	reply.Root = a.vm.Root()
	log.Infow("ApplierService.ApplySignedMessage", "args", args, "reply", reply)
	return nil
}

func (a *VmWrapperService) ApplyTipSetMessages(r *http.Request, args *ApplyTipSetMessagesArgs, reply *ApplyTipSetMessagesReply) error {
	result, err := a.applier.ApplyTipSetMessages(args.Epoch, args.Blocks, drivers.NewRandomnessSource())
	if err != nil {
		return err
	}
	reply.Receipts = result.Receipts
	reply.Root = a.vm.Root()
	log.Infow("ApplierService.ApplyTipSetMessages", "args", args, "reply", reply)
	return nil
}

func NewVmWrapperService(f state.Factories) *VmWrapperService {
	stateWrapper, applier := f.NewStateAndApplier()
	return &VmWrapperService{
		factory: f,
		vm:      stateWrapper,
		applier: applier,
	}
}

type VmWrapperService struct {
	factory state.Factories
	vm      state.VMWrapper
	applier state.Applier
}

type RootReply struct {
	Root cid.Cid
}

func (v *VmWrapperService) Root(r *http.Request, args *struct{}, reply *RootReply) error {
	reply.Root = v.vm.Root()
	log.Infow("Root", "reply", reply)
	return nil
}

type StoreGetArgs struct {
	Key cid.Cid
}

type StoreGetReply struct {
	Out []byte
}

func (v *VmWrapperService) StoreGet(r *http.Request, args *StoreGetArgs, reply *StoreGetReply) error {
	var out cbg.Deferred
	if err := v.vm.StoreGet(args.Key, &out); err != nil {
		return err
	}
	reply.Out = out.Raw
	log.Infow("StoreGet", "args", args, "reply", reply)
	return nil
}

type StorePutArgs struct {
	Value []byte
}

type StorePutReply struct {
	Key cid.Cid
}

func (v *VmWrapperService) StorePut(r *http.Request, args *StorePutArgs, reply *StorePutReply) error {
	key, err := v.vm.StorePut(runtime.CBORBytes(args.Value))
	if err != nil {
		return err
	}
	reply.Key = key
	log.Infow("StorePut", "args", args, "reply", reply)
	return nil
}

type ActorArgs struct {
	Addr address.Address
}

type ActorReply struct {
	Code       cid.Cid
	Head       cid.Cid
	Balance    big.Int
	CallSeqNum uint64
}

func (v *VmWrapperService) Actor(r *http.Request, args *ActorArgs, reply *ActorReply) error {
	actor, err := v.vm.Actor(args.Addr)
	if err != nil {
		return err
	}
	reply.Code = actor.Code()
	reply.Head = actor.Head()
	reply.Balance = actor.Balance()
	reply.CallSeqNum = actor.CallSeqNum()
	log.Infow("Actor", "args", args, "reply", reply)
	return nil
}

type SetActorStateArgs struct {
	Addr    address.Address
	Balance abi.TokenAmount
	State   []byte // TODO see if you can make this CBORBytes instead
}

func (v *VmWrapperService) SetActorState(r *http.Request, args *SetActorStateArgs, reply *ActorReply) error {
	actor, err := v.vm.SetActorState(args.Addr, args.Balance, runtime.CBORBytes(args.State))
	if err != nil {
		return err
	}
	reply.Code = actor.Code()
	reply.Head = actor.Head()
	reply.Balance = actor.Balance()
	reply.CallSeqNum = actor.CallSeqNum()
	log.Infow("SetActorState", "args", args, "reply", reply)
	return nil
}

type CreateActorArgs struct {
	Code    cid.Cid
	Addr    address.Address
	Balance abi.TokenAmount
	State   []byte
}

type CreateActorReply struct {
	Addr       address.Address
	Code       cid.Cid
	Head       cid.Cid
	Balance    big.Int
	CallSeqNum uint64
}

func (v *VmWrapperService) CreateActor(r *http.Request, args *CreateActorArgs, reply *CreateActorReply) error {
	actor, addr, err := v.vm.CreateActor(args.Code, args.Addr, args.Balance, runtime.CBORBytes(args.State))
	if err != nil {
		return err
	}
	reply.Addr = addr
	reply.Code = actor.Code()
	reply.Head = actor.Head()
	reply.Balance = actor.Balance()
	reply.CallSeqNum = actor.CallSeqNum()
	log.Infow("CreateActor", "args", args, "reply", reply)
	return nil
}

func (v *VmWrapperService) NewVM(r *http.Request, args *struct{}, reply *struct{}) error {
	wrapper, applier := v.factory.NewStateAndApplier()
	v.vm = wrapper
	v.applier = applier
	log.Info("NEWVM")
	return nil
}

func main() {
	s := rpc.NewServer()
	s.RegisterCodec(jsonrpc.NewCodec(), "application/json")

	if err := s.RegisterService(new(ConfigService), ""); err != nil {
		log.Fatal(err)
	}

	f := vmcontext.NewFactories(&vmcontext.ValidationConfig{
		TrackGas:         true,
		CheckExitCode:    true,
		CheckReturnValue: true,
		CheckStateRoot:   true,
	})
	if err := s.RegisterService(NewVmWrapperService(f), ""); err != nil {
		log.Fatal(err)
	}
	http.Handle("/rpc", s)
	if err := http.ListenAndServe("127.0.0.1:8378", nil); err != nil {
		panic(err)
	}
}
