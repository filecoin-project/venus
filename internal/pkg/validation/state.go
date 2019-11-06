package validation

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/storagemarket"
	"github.com/pkg/errors"
	"math/rand"

	vstate "github.com/filecoin-project/chain-validation/pkg/state"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

type StateWrapper struct {
	state.Tree
	vm.StorageMap
	keys *keyStore
}

var _ vstate.Wrapper = &StateWrapper{}

func NewState() *StateWrapper {
	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	cst := hamt.CSTFromBstore(bs)
	treeImpl := state.NewEmptyStateTree(cst)
	storageImpl := vm.NewStorageMap(bs)
	return &StateWrapper{treeImpl, storageImpl, newKeyStore()}
}

func (s *StateWrapper) Cid() cid.Cid {
	panic("implement me")
}

func (s *StateWrapper) Actor(addr vstate.Address) (vstate.Actor, error) {
	vaddr, err := address.NewFromBytes([]byte(addr))
	if err != nil {
		return nil, err
	}
	fcActor, err := s.Tree.GetActor(context.TODO(), vaddr)
	if err != nil {
		return nil, err
	}
	return &actorWrapper{*fcActor}, nil
}

func (s *StateWrapper) Storage(addr vstate.Address) (vstate.Storage, error) {
	addrInt, err := address.NewFromBytes([]byte(addr))
	if err != nil {
		return nil, err
	}

	actor, err := s.Tree.GetActor(context.TODO(), addrInt)
	if err != nil {
		return nil, err
	}

	storageInt := s.StorageMap.NewStorage(addrInt, actor)
	// The internal storage implements vstate.Storage directly for now.
	return storageInt, nil
}

func (s *StateWrapper) NewAccountAddress() (vstate.Address, error) {
	return s.keys.newAddress()
}

func (s *StateWrapper) SetActor(addr vstate.Address, code vstate.ActorCodeID, balance vstate.AttoFIL) (vstate.Actor, vstate.Storage, error) {
	ctx := context.TODO()
	addrInt, err := address.NewFromBytes([]byte(addr))
	if err != nil {
		return nil, nil, err
	}
	actr := &actorWrapper{actor.Actor{
		Code:    fromActorCode(code),
		Balance: types.NewAttoFIL(balance),
	}}
	if err := s.Tree.SetActor(ctx, addrInt, &actr.Actor); err != nil {
		return nil, nil, err
	}
	_, err = s.Tree.Flush(ctx)
	if err != nil {
		return nil, nil, err
	}

	storage := s.NewStorage(addrInt, &actr.Actor)
	return actr, storage, nil
}

func (s *StateWrapper) SetSingletonActor(addr vstate.SingletonActorID, balance vstate.AttoFIL) (vstate.Actor, vstate.Storage, error) {
	ctx := context.Background()
	vaddr := fromSingletonAddress(addr)
	fcAddr, err := address.NewFromBytes([]byte(vaddr))
	if err != nil {
		return nil, nil, err
	}

	switch fcAddr {
	case address.InitAddress:
		intAct := initactor.NewActor()
		err = (&initactor.Actor{}).InitializeState(s.StorageMap.NewStorage(address.InitAddress, intAct), "localnet")
		if err != nil {
			return nil, nil, err
		}
		if err := s.Tree.SetActor(ctx, fcAddr, intAct); err != nil {
			return nil, nil, err
		}
		_, err = s.Tree.Flush(ctx)
		if err != nil {
			return nil, nil, err
		}

		storage := s.NewStorage(fcAddr, intAct)
		return &actorWrapper{*intAct}, storage, nil
	case address.StorageMarketAddress:
		stAct := storagemarket.NewActor()
		err := (&storagemarket.Actor{}).InitializeState(s.StorageMap.NewStorage(address.StorageMarketAddress, stAct), types.TestProofsMode)
		if err != nil {
			return nil, nil, err
		}
		if err := s.Tree.SetActor(ctx, fcAddr, stAct); err != nil {
			return nil, nil, err
		}
		_, err = s.Tree.Flush(ctx)
		if err != nil {
			return nil, nil, err
		}
		storage := s.NewStorage(fcAddr, stAct)
		return &actorWrapper{*stAct}, storage, nil
	case address.BurntFundsAddress:
		bal := types.NewAttoFIL(balance)
		fcActor := &actor.Actor{
			Code:    types.AccountActorCodeCid,
			Balance: bal,
			Head:    cid.Undef,
		}
		if err := s.Tree.SetActor(ctx,address.BurntFundsAddress, fcActor); err != nil {
			return nil, nil, errors.Wrapf(err, "set burntfunds actor")
		}
		_, err = s.Tree.Flush(ctx)
		if err != nil {
			return nil, nil, err
		}
		storage := s.NewStorage(fcAddr, fcActor)
		return &actorWrapper{*fcActor}, storage, nil
	case address.NetworkAddress:
		bal := types.NewAttoFIL(balance)
		fcActor := &actor.Actor{
			Code:    types.AccountActorCodeCid,
			Balance: bal,
			Head:    cid.Undef,
		}
		if err := s.Tree.SetActor(ctx,address.NetworkAddress, fcActor); err != nil {
			return nil, nil, errors.Wrapf(err, "set network actor")
		}
		_, err = s.Tree.Flush(ctx)
		if err != nil {
			return nil, nil, err
		}
		storage := s.NewStorage(fcAddr, fcActor)
		return &actorWrapper{*fcActor}, storage, nil
		// TODO need to add StoragePowerActor when go-filecoin supports it
	default:
		return nil, nil, errors.Errorf("%v is not a singleton actor address", addr)
	}

}

func (s *StateWrapper) Signer() *keyStore {
	return s.keys
}

//
// Key store
//

type keyStore struct {
	// Private keys by address
	keys map[address.Address]*types.KeyInfo
	// Seed for deterministic key generation.
	seed int64
}

func newKeyStore() *keyStore {
	return &keyStore{
		keys: make(map[address.Address]*types.KeyInfo),
		seed: 0,
	}
}

func (s *keyStore) newAddress() (vstate.Address, error) {
	randSrc := rand.New(rand.NewSource(s.seed))
	prv, err := crypto.GenerateKeyFromSeed(randSrc)
	if err != nil {
		return "", err
	}

	ki := &types.KeyInfo{
		PrivateKey:  prv,
		CryptSystem: "secp256k1",
	}
	addr, err := ki.Address()
	if err != nil {
		return "", err
	}
	s.keys[addr] = ki
	s.seed++
	return vstate.Address(addr.Bytes()), nil
}

// FIXME this only signes secp, need to suuport bls too.
func (as *keyStore) SignBytes(data []byte, addr address.Address) (types.Signature, error) {
	ki, ok := as.keys[addr]
	if !ok {
		return types.Signature{}, fmt.Errorf("unknown address %v", addr)
	}
	return crypto.SignSecp(ki.Key(), data)
}

//
// Actor Wrapper
//

type actorWrapper struct {
	actor.Actor
}

func (a *actorWrapper) Code() cid.Cid {
	return a.Actor.Code
}

func (a *actorWrapper) Head() cid.Cid {
	return a.Actor.Head
}

func (a *actorWrapper) Nonce() uint64 {
	return uint64(a.Actor.Nonce)
}

func (a *actorWrapper) Balance() vstate.AttoFIL {
	return a.Actor.Balance.AsBigInt()
}

func fromActorCode(code vstate.ActorCodeID) cid.Cid {
	switch code {
	case vstate.AccountActorCodeCid:
		return types.AccountActorCodeCid
	case vstate.StorageMinerCodeCid:
		return types.StorageMarketActorCodeCid
	case vstate.MultisigActorCodeCid:
		panic("nyi")
	case vstate.PaymentChannelActorCodeCid:
		return types.PaymentBrokerActorCodeCid
	default:
		panic(fmt.Errorf("unknown actor code: %v", code))
	}
}

func fromSingletonAddress(addr vstate.SingletonActorID) vstate.Address {
	switch addr {
	case vstate.InitAddress:
		return vstate.Address(address.InitAddress.Bytes())
	case vstate.NetworkAddress:
		return vstate.Address(address.NetworkAddress.Bytes())
	case vstate.StorageMarketAddress:
		return vstate.Address(address.StorageMarketAddress.Bytes())
	case vstate.BurntFundsAddress:
		return vstate.Address(address.BurntFundsAddress.Bytes())
	default:
		panic(fmt.Errorf("unknown singleton actor address: %v", addr))
	}
}
