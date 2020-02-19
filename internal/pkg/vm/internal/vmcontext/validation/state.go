package validation

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/minio/blake2b-simd"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"

	ffi "github.com/filecoin-project/filecoin-ffi"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-crypto"

	big_spec "github.com/filecoin-project/specs-actors/actors/abi/big"
	builtin_spec "github.com/filecoin-project/specs-actors/actors/builtin"
	crypto_spec "github.com/filecoin-project/specs-actors/actors/crypto"

	vstate "github.com/filecoin-project/chain-validation/state"

	"github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func init() {
	cst := cbor.NewMemCborStore()
	emptyobject, err := cst.Put(context.TODO(), map[string]string{})
	if err != nil {
		panic(err)
	}

	EmptyObjectCid = emptyobject
}

var EmptyObjectCid cid.Cid

var _ vstate.Wrapper = &StateWrapper{}

type StateWrapper struct {
	// The blockstore underlying the state tree and storage.
	bs blockstore.Blockstore

	// HAMT-CBOR store on top of the blockstore.
	storage *ctxStorage

	// A store for encryption keys.
	keys *keyStore

	// CID of the root of the state tree.
	stateRoot cid.Cid
}

func NewState() *StateWrapper {
	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	cst := cbor.NewCborStore(bs)
	// Put EmptyObjectCid value in the store. When an actor is initially created its Head is set to this value.
	_, err := cst.Put(context.TODO(), map[string]string{})
	if err != nil {
		panic(err)
	}

	treeImpl := state.NewTree(cst)
	root, err := treeImpl.Flush(context.TODO())
	if err != nil {
		panic(err)
	}
	return &StateWrapper{bs, newCtxStorage(cst), newKeyStore(), root}
}

func (s StateWrapper) Cid() cid.Cid {
	return s.stateRoot
}

func (s StateWrapper) Actor(address address.Address) (vstate.Actor, error) {
	ctx := context.TODO()
	tree, err := state.NewTreeLoader().LoadStateTree(ctx, s.storage, s.stateRoot)
	if err != nil {
		return nil, err
	}
	fcActor, err := tree.GetActor(ctx, address)
	if err != nil {
		return nil, err
	}
	return &actorWrapper{*fcActor}, nil
}

func (s StateWrapper) SetActor(addr address.Address, code cid.Cid, balance big_spec.Int) (vstate.Actor, vstate.Storage, error) {
	ctx := context.TODO()
	tree, err := state.NewTreeLoader().LoadStateTree(ctx, s.storage, s.stateRoot)
	if err != nil {
		return nil, nil, err
	}
	actr := &actorWrapper{actor.Actor{
		Code:    enccid.NewCid(code),
		Balance: balance,
		Head:    enccid.NewCid(EmptyObjectCid),
	}}
	// FIXME: this call will bypass the init actor, which is probably(?) not what we want
	// lotus has a method called RegisterActor that calls into the init actor and creates the actor correctly using the init actor.
	// go-filecoin will probably(?) need to move to creating actors as lotus does.
	if err := tree.SetActor(ctx, addr, &actr.Actor); err != nil {
		return nil, nil, fmt.Errorf("register new address for actor: %w", err)
	}
	return actr, s.storage, s.flush(tree)
}

func (s StateWrapper) SetSingletonActor(addr address.Address, balance big_spec.Int) (vstate.Actor, vstate.Storage, error) {
	panic("TODO")
	// TODO once ignacio lands the new gengen code that works with the nex specs actor use it here.
	switch addr {
	case builtin_spec.InitActorAddr:
	case builtin_spec.StorageMarketActorAddr:
	case builtin_spec.StoragePowerActorAddr:
	case builtin_spec.RewardActorAddr:
	case builtin_spec.BurntFundsActorAddr:
	default:
		return nil, nil, fmt.Errorf("%v is not a singleton actor address", addr)
	}
	return nil, nil, nil
}

func (s StateWrapper) Storage() (vstate.Storage, error) {
	return s.storage, nil
}

func (s StateWrapper) NewSecp256k1AccountAddress() address.Address {
	return s.keys.NewSecp256k1Address()
}

func (s StateWrapper) NewBLSAccountAddress() address.Address {
	return s.keys.NewBLSAddress()
}

func (s StateWrapper) Sign(ctx context.Context, addr address.Address, data []byte) (*crypto_spec.Signature, error) {
	panic("implement me")
}

// Flushes a state tree to storage and sets this state's root to that tree's root CID.
func (s *StateWrapper) flush(tree state.Tree) (err error) {
	s.stateRoot, err = tree.Flush(context.TODO())
	return
}

//
// Actor Wrapper
//

type actorWrapper struct {
	actor.Actor
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

func (a *actorWrapper) Balance() big_spec.Int {
	return a.Actor.Balance

}

//
// Storage
//

type ctxStorage struct {
	cbor.IpldStore
	ctx context.Context
}

func newCtxStorage(cst cbor.IpldStore) *ctxStorage {
	return &ctxStorage{
		IpldStore: cst,
		ctx:       context.TODO(),
	}
}

func (cs *ctxStorage) Context() context.Context {
	return cs.ctx
}

//
// Key store
//
type keyStore struct {
	// Private keys by address
	keys map[address.Address]*types.KeyInfo
	// Seed for deterministic secp key generation.
	seed int64
}

func newKeyStore() *keyStore {
	return &keyStore{
		keys: make(map[address.Address]*types.KeyInfo),
		seed: 0,
	}
}

func (s *keyStore) newSecp256k1Key() *types.KeyInfo {
	randSrc := rand.New(rand.NewSource(s.seed))
	prv, err := crypto.GenerateKeyFromSeed(randSrc)
	if err != nil {
		panic(err)
	}
	ki := &types.KeyInfo{
		CryptSystem: types.SECP256K1,
		PrivateKey:  prv,
	}
	return ki
}

func (s *keyStore) newBLSKey() *types.KeyInfo {
	sk := ffi.PrivateKeyGenerate()
	ki := &types.KeyInfo{
		CryptSystem: types.BLS,
		PrivateKey:  sk[:],
	}
	return ki
}

func (s *keyStore) NewSecp256k1Address() address.Address {
	secpKey := s.newSecp256k1Key()
	addr, err := secpKey.Address()
	if err != nil {
		panic(err)
	}
	s.keys[addr] = secpKey
	s.seed++
	return addr
}

func (s *keyStore) NewBLSAddress() address.Address {
	blsKey := s.newBLSKey()
	addr, err := blsKey.Address()
	if err != nil {
		panic(err)
	}
	s.keys[addr] = blsKey
	// bls key generation does not accept a seed :(
	return addr
}

func (s *keyStore) Sign(ctx context.Context, addr address.Address, data []byte) (types.Signature, error) {
	ki, ok := s.keys[addr]
	if !ok {
		return nil, fmt.Errorf("unknown address %v", addr)
	}
	switch ki.Type() {
	case types.BLS:
		var pk ffi.PrivateKey
		copy(pk[:], ki.PrivateKey)
		digest := ffi.PrivateKeySign(pk, data)
		return digest[:], nil
	case types.SECP256K1:
		b2sum := blake2b.Sum256(data)
		digest, err := crypto.Sign(ki.PrivateKey, b2sum[:])
		if err != nil {
			return nil, err
		}
		return digest, nil
	default:
		return nil, fmt.Errorf("unknown key type: %s for address: %s", ki.Type, addr)
	}
}
