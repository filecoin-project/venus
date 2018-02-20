package core

import (
	"fmt"
	"math/big"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cbor "gx/ipfs/QmZpue627xQuNGXn7xHieSjSZ8N4jot6oBHwe9XTn3e4NU/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(TokenStorage{})
	cbor.RegisterCborType(Balance{})
}

var (
	// ErrRequiredFrom is returned when a method requires a from address, but none was passed.
	ErrRequiredFrom = errors.New("message is missing required from")
)

// TokenActor is the builtin actor for handling account balances.
//
// TokenActor __is__ a singleton actor, that should only exist at one location
// and manages the balances of all accounts.
type TokenActor struct{}

// TokenStorage is what the TokenActor uses to store data permanently
// onchain. It is unmarshalled & marshalled when needed, as only raw bytes
// can be stored onchain.
type TokenStorage struct {
	Balances map[types.Address]*Balance
}

// Balance represents the balance sheet for an individual account.
type Balance struct {
	// Total is the amount of filecoin this account holds.
	Total *big.Int
}

// ensure TokenActor is an ExecutableActor at compile time
var _ ExecutableActor = (*TokenActor)(nil)

// NewTokenActor creates a new Actor with a predefined balance.
func NewTokenActor(balances map[types.Address]*Balance) (*types.Actor, error) {
	storageBytes, err := MarshalStorage(&TokenStorage{
		Balances: balances,
	})
	if err != nil {
		return nil, err
	}
	return types.NewActorWithMemory(types.TokenActorCid, storageBytes), nil
}

// tokenExports are the publicly (externally callable) methods of the TokenActor.
var tokenExports = Exports{
	"balance": &FunctionSignature{
		Params: []interface{}{types.Address("")},
		Return: &big.Int{},
	},
	"transfer": &FunctionSignature{
		Params: []interface{}{types.Address(""), &big.Int{}},
		Return: nil,
	},
}

// Exports makes the available methods for this contract available.
func (state *TokenActor) Exports() Exports {
	return tokenExports
}

// Balance retrieves the current balance of this token in Filecoin.
func (state *TokenActor) Balance(ctx *VMContext, id types.Address) (*big.Int, uint8, error) {
	storage, err := loadStorage(ctx)
	if err != nil {
		return nil, 1, err
	}

	b, ok := storage.Balances[id]
	if !ok {
		return big.NewInt(0), 0, nil
	}

	return b.Total, 0, nil
}

// Transfer sends a specified amount of Filecoin from the sender to the receiver of this token.
func (state *TokenActor) Transfer(ctx *VMContext, receiver types.Address, amount *big.Int) (uint8, error) {
	if !ctx.Message().HasFrom() {
		return 1, ErrRequiredFrom
	}

	sender := ctx.Message().From()

	_, err := withTokenStorage(ctx, func(storage *TokenStorage) (interface{}, error) {
		senderBalance, ok := storage.Balances[sender]
		if !ok {
			return 1, fmt.Errorf("no balance available for sender: %s", sender)
		}

		receiverBalance, ok := storage.Balances[receiver]
		if !ok {
			storage.Balances[receiver] = &Balance{Total: big.NewInt(0)}
			receiverBalance = storage.Balances[receiver]
		}

		// make sure enough is available
		if senderBalance.Total.Cmp(amount) == -1 {
			return 1, fmt.Errorf("not enough balance")
		}

		senderBalance.Total.Sub(senderBalance.Total, amount)
		receiverBalance.Total.Add(receiverBalance.Total, amount)
		return nil, nil
	})

	if err != nil {
		return 1, err
	}

	return 0, nil
}

// withTokenStorage is a helper to initialize the tokens storage, operate on it, and then
// commit it again.
func withTokenStorage(ctx *VMContext, f func(*TokenStorage) (interface{}, error)) (interface{}, error) {
	storage, err := loadStorage(ctx)
	if err != nil {
		return nil, err
	}

	ret, err := f(storage)
	if err != nil {
		return nil, err
	}

	newStorage, err := MarshalStorage(storage)
	if err != nil {
		return nil, err
	}

	if err := ctx.WriteStorage(newStorage); err != nil {
		return nil, err
	}

	return ret, nil
}

// loadStorage fetches the storage from the actor.
func loadStorage(ctx *VMContext) (*TokenStorage, error) {
	return unmarshalTokenStorage(ctx.ReadStorage())
}

// unmarshalTokenStorage initializes and unmarshales the token storage.
func unmarshalTokenStorage(raw []byte) (*TokenStorage, error) {
	storage := &TokenStorage{Balances: map[types.Address]*Balance{}}

	// no storage to initialize
	if len(raw) == 0 {
		return storage, nil
	}

	if err := UnmarshalStorage(raw, storage); err != nil {
		return nil, err
	}

	return storage, nil
}
