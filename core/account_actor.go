package core

import (
	"fmt"
	"math/big"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cbor "gx/ipfs/QmZpue627xQuNGXn7xHieSjSZ8N4jot6oBHwe9XTn3e4NU/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(AccountMemory{})
}

type AccountActor struct{}

type AccountMemory struct {
	Balance *big.Int
}

func (mem *AccountMemory) isMemory() bool {
	return true
}

var _ ExecutableActor = (*AccountActor)(nil)
var _ ActorMemory = (*AccountMemory)(nil)

// NewAccountActor creates a new Actor with a predefined balance.
func NewAccountActor(balance *big.Int) (*types.Actor, error) {
	memoryBytes, err := MarshalMemory(&AccountMemory{
		Balance: balance,
	})
	if err != nil {
		return nil, err
	}
	return types.NewActorWithMemory(types.AccountActorCid, memoryBytes), nil
}

var exports = Exports{
	"balance": &FunctionSignature{
		Params: []interface{}{},
		Return: &big.Int{},
	},
	"transfer": &FunctionSignature{
		Params: []interface{}{},
		Return: nil,
	},
	"subtract": &FunctionSignature{
		Params: []interface{}{&types.Message{}, &big.Int{}},
		Return: nil,
	},
	"main": &FunctionSignature{
		Params: []interface{}{},
		Return: nil,
	},
}

// Execute is the entry point for calling from the external land into the VM.
func (account *AccountActor) Execute(ctx *VMContext) ([]byte, uint8, error) {
	memory, err := account.unmarshalMemory(ctx.ReadStorage())
	if err != nil {
		return nil, 1, err
	}

	ret, exitCode, err := MakeTypedExport(account, ctx.Message().Method())(ctx, memory)
	if err != nil {
		return ret, exitCode, err
	}

	newMemory, err := account.marshalMemory(memory)
	if err != nil {
		return nil, 1, err
	}

	if err := ctx.WriteStorage(newMemory); err != nil {
		return nil, 1, err
	}

	return ret, exitCode, nil
}

// Exports makes the available methods for this contract available.
func (account *AccountActor) Exports() Exports {
	return exports
}

func (account *AccountActor) Main(ctx *VMContext, memory *AccountMemory) (uint8, error) {
	return 0, nil
}

func (account *AccountActor) Balance(ctx *VMContext, memory *AccountMemory) (*big.Int, uint8, error) {
	return memory.Balance, 0, nil
}

func (account *AccountActor) Transfer(ctx *VMContext, memory *AccountMemory) (uint8, error) {
	value := ctx.Message().Value()
	_, _, err := ctx.Send(ctx.Message().From(), "subtract", []interface{}{ctx.Message(), value})
	if err != nil {
		return 1, errors.Wrap(err, "failed to send message: subtract")
	}

	memory.Balance.Add(memory.Balance, value)

	return 0, nil
}

func (account *AccountActor) Subtract(ctx *VMContext, memory *AccountMemory, msg *types.Message, value *big.Int) (uint8, error) {
	// validate we agreed to this value being sent
	// TODO: instead of passing the message, only send the cid, and fetch it from the state, to make sure it
	// is valid and included in the state tree
	// how can we prevent reuse?
	// 1. validate signature
	// TODO
	// 2. check we are the sender
	if msg.From() != ctx.Message().To() {
		return 1, fmt.Errorf("invalid sender")
	}
	// 3. check the value matches
	if value.Cmp(msg.Value()) != 0 {
		return 1, fmt.Errorf("invalid value")
	}

	// make sure enough is available
	if memory.Balance.Cmp(value) == -1 {
		return 1, fmt.Errorf("not enough balance")
	}

	memory.Balance.Sub(memory.Balance, value)

	return 0, nil
}

func (account *AccountActor) unmarshalMemory(raw []byte) (*AccountMemory, error) {
	memory := &AccountMemory{Balance: big.NewInt(0)}

	// no memory to initialize
	if len(raw) == 0 {
		return memory, nil
	}

	if err := UnmarshalMemory(raw, memory); err != nil {
		return nil, err
	}

	return memory, nil
}

func (account *AccountActor) marshalMemory(memory *AccountMemory) ([]byte, error) {
	return MarshalMemory(memory)
}
