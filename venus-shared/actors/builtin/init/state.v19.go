// FETCHED FROM LOTUS: builtin/init/state.go.template

package init

import (
	"crypto/sha256"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"

	actorstypes "github.com/filecoin-project/go-state-types/actors"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"

	"github.com/filecoin-project/go-state-types/manifest"

	builtin19 "github.com/filecoin-project/go-state-types/builtin"
	init19 "github.com/filecoin-project/go-state-types/builtin/v19/init"
	adt19 "github.com/filecoin-project/go-state-types/builtin/v19/util/adt"
)

var _ State = (*state19)(nil)

func load19(store adt.Store, root cid.Cid) (State, error) {
	out := state19{store: store}
	err := store.Get(store.Context(), root, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func make19(store adt.Store, networkName string) (State, error) {
	out := state19{store: store}

	s, err := init19.ConstructState(store, networkName)
	if err != nil {
		return nil, err
	}

	out.State = *s

	return &out, nil
}

type state19 struct {
	init19.State
	store adt.Store
}

func (s *state19) ResolveAddress(address address.Address) (address.Address, bool, error) {
	return s.State.ResolveAddress(s.store, address)
}

func (s *state19) MapAddressToNewID(address address.Address) (address.Address, error) {
	return s.State.MapAddressToNewID(s.store, address)
}

func (s *state19) ForEachActor(cb func(id abi.ActorID, address address.Address) error) error {
	addrs, err := adt19.AsMap(s.store, s.State.AddressMap, builtin19.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	var actorID cbg.CborInt
	return addrs.ForEach(&actorID, func(key string) error {
		addr, err := address.NewFromBytes([]byte(key))
		if err != nil {
			return err
		}
		return cb(abi.ActorID(actorID), addr)
	})
}

func (s *state19) NetworkName() (string, error) {
	return string(s.State.NetworkName), nil
}

func (s *state19) SetNetworkName(name string) error {
	s.State.NetworkName = name
	return nil
}

func (s *state19) SetNextID(id abi.ActorID) error {
	s.State.NextID = id
	return nil
}

func (s *state19) Remove(addrs ...address.Address) (err error) {
	m, err := adt19.AsMap(s.store, s.State.AddressMap, builtin19.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	for _, addr := range addrs {
		if err = m.Delete(abi.AddrKey(addr)); err != nil {
			return fmt.Errorf("failed to delete entry for address: %s; err: %w", addr, err)
		}
	}
	amr, err := m.Root()
	if err != nil {
		return fmt.Errorf("failed to get address map root: %w", err)
	}
	s.State.AddressMap = amr
	return nil
}

func (s *state19) SetAddressMap(mcid cid.Cid) error {
	s.State.AddressMap = mcid
	return nil
}

func (s *state19) GetState() interface{} {
	return &s.State
}

func (s *state19) AddressMap() (adt.Map, error) {
	return adt19.AsMap(s.store, s.State.AddressMap, builtin19.DefaultHamtBitwidth)
}

func (s *state19) AddressMapBitWidth() int {
	return builtin19.DefaultHamtBitwidth
}

func (s *state19) AddressMapHashFunction() func(input []byte) []byte {
	return func(input []byte) []byte {
		res := sha256.Sum256(input)
		return res[:]
	}
}

func (s *state19) ActorKey() string {
	return manifest.InitKey
}

func (s *state19) ActorVersion() actorstypes.Version {
	return actorstypes.Version19
}

func (s *state19) Code() cid.Cid {
	code, ok := actors.GetActorCodeID(s.ActorVersion(), s.ActorKey())
	if !ok {
		panic(fmt.Errorf("didn't find actor %v code id for actor version %d", s.ActorKey(), s.ActorVersion()))
	}

	return code
}
