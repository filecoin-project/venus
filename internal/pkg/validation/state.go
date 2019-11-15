package validation

import (
	"context"
	"fmt"
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
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// StateWrapper provides a wrapper for a state tree, storage, map and keystore.
type StateWrapper struct {
	vm.StorageMap
	keys *keyStore

	stateRoot cid.Cid
	cst       *hamt.CborIpldStore
}

var _ vstate.Wrapper = &StateWrapper{}

// NewState returns a StateWrapper.
func NewState() *StateWrapper {
	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	cst := hamt.CSTFromBstore(bs)
	treeImpl := state.NewTree(cst)
	storageImpl := vm.NewStorageMap(bs)
	stateRoot, err := treeImpl.Flush(context.TODO())
	if err != nil {
		panic(err)
	}
	return &StateWrapper{storageImpl, newKeyStore(), stateRoot, cst}
}

// Cid returns the cid of the state wrappers current state
func (s *StateWrapper) Cid() cid.Cid {
	return s.stateRoot
}

// Actor returns the actor whos address is `addr`.
func (s *StateWrapper) Actor(addr vstate.Address) (vstate.Actor, error) {
	vaddr, err := address.NewFromBytes([]byte(addr))
	if err != nil {
		return nil, err
	}

	tree, err := state.NewTreeLoader().LoadStateTree(context.TODO(), s.cst, s.stateRoot)
	if err != nil {
		return nil, err
	}

	fcActor, err := tree.GetActor(context.TODO(), vaddr)
	if err != nil {
		return nil, err
	}
	return &actorWrapper{*fcActor}, nil
}

// Storage returns the storage for actor at address `addr`.
func (s *StateWrapper) Storage(addr vstate.Address) (vstate.Storage, error) {
	addrInt, err := address.NewFromBytes([]byte(addr))
	if err != nil {
		return nil, err
	}

	tree, err := state.NewTreeLoader().LoadStateTree(context.TODO(), s.cst, s.stateRoot)
	if err != nil {
		return nil, err
	}

	fcActor, err := tree.GetActor(context.TODO(), addrInt)
	if err != nil {
		return nil, err
	}

	storageInt := s.StorageMap.NewStorage(addrInt, fcActor)
	// The internal storage implements vstate.Storage directly for now.
	return storageInt, nil
}

// NewAccountAddress returns an account actor address.
func (s *StateWrapper) NewAccountAddress() (vstate.Address, error) {
	return s.keys.newAddress()
}

// SetActor sets an actor in the state tree.
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

	tree, err := state.NewTreeLoader().LoadStateTree(context.TODO(), s.cst, s.stateRoot)
	if err != nil {
		return nil, nil, err
	}

	if err := tree.SetActor(ctx, addrInt, &actr.Actor); err != nil {
		return nil, nil, err
	}

	s.stateRoot, err = tree.Flush(ctx)
	if err != nil {
		return nil, nil, err
	}

	storage := s.NewStorage(addrInt, &actr.Actor)
	return actr, storage, nil
}

// SetSingletonActor sets a singleton actor in the state tree.
func (s *StateWrapper) SetSingletonActor(addr vstate.SingletonActorID, balance vstate.AttoFIL) (vstate.Actor, vstate.Storage, error) {
	ctx := context.Background()
	vaddr := fromSingletonAddress(addr)
	fcAddr, err := address.NewFromBytes([]byte(vaddr))
	if err != nil {
		return nil, nil, err
	}

	switch fcAddr {
	case address.InitAddress:
		fcActor := initactor.NewActor()
		err = (&initactor.Actor{}).InitializeState(s.StorageMap.NewStorage(address.InitAddress, fcActor), "localnet")
		if err != nil {
			return nil, nil, err
		}
		if err := s.setActorAndStateRoot(ctx, fcAddr, fcActor); err != nil {
			return nil, nil, err
		}
		storage := s.NewStorage(fcAddr, fcActor)
		return &actorWrapper{*fcActor}, storage, nil
	case address.StorageMarketAddress:
		fcActor := storagemarket.NewActor()
		err := (&storagemarket.Actor{}).InitializeState(s.StorageMap.NewStorage(address.StorageMarketAddress, fcActor), types.TestProofsMode)
		if err != nil {
			return nil, nil, err
		}
		if err := s.setActorAndStateRoot(ctx, fcAddr, fcActor); err != nil {
			return nil, nil, err
		}
		storage := s.NewStorage(fcAddr, fcActor)
		return &actorWrapper{*fcActor}, storage, nil
	case address.BurntFundsAddress:
		bal := types.NewAttoFIL(balance)
		fcActor := &actor.Actor{
			Code:    types.AccountActorCodeCid,
			Balance: bal,
			Head:    cid.Undef,
		}
		if err := s.setActorAndStateRoot(ctx, address.BurntFundsAddress, fcActor); err != nil {
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
		if err := s.setActorAndStateRoot(ctx, address.NetworkAddress, fcActor); err != nil {
			return nil, nil, err
		}
		storage := s.NewStorage(fcAddr, fcActor)
		return &actorWrapper{*fcActor}, storage, nil
		// TODO need to add StoragePowerActor when go-filecoin supports it
	default:
		return nil, nil, errors.Errorf("%v is not a singleton actor address", addr)
	}

}

func (s *StateWrapper) setActorAndStateRoot(ctx context.Context, addr address.Address, act *actor.Actor) error {
	tree, err := state.NewTreeLoader().LoadStateTree(context.TODO(), s.cst, s.stateRoot)
	if err != nil {
		return err
	}

	if err := tree.SetActor(ctx, addr, act); err != nil {
		return errors.Wrapf(err, "set network actor")
	}
	s.stateRoot, err = tree.Flush(ctx)
	if err != nil {
		return err
	}
	return nil
}

// Signer returns a signer
func (s *StateWrapper) Signer() types.Signer {
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

// SignBytes signes data with the public key in addr.
// FIXME this only signes secp, need to suuport bls too.
func (s *keyStore) SignBytes(data []byte, addr address.Address) (types.Signature, error) {
	ki, ok := s.keys[addr]
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

// Code returns the actor code CID.
func (a *actorWrapper) Code() cid.Cid {
	return a.Actor.Code
}

// Head returns the actor Head CID.
func (a *actorWrapper) Head() cid.Cid {
	return a.Actor.Head
}

// Nonce returns the actor nonce.
func (a *actorWrapper) Nonce() uint64 {
	return uint64(a.Actor.Nonce)
}

// Balance returns the actor balance.
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
