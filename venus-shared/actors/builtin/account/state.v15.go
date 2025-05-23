// FETCHED FROM LOTUS: builtin/account/state.go.template

package account

import (
	"fmt"

	actorstypes "github.com/filecoin-project/go-state-types/actors"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-state-types/manifest"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"

	account15 "github.com/filecoin-project/go-state-types/builtin/v15/account"
)

var _ State = (*state15)(nil)

func load15(store adt.Store, root cid.Cid) (State, error) {
	out := state15{store: store}
	err := store.Get(store.Context(), root, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func make15(store adt.Store, addr address.Address) (State, error) {
	out := state15{store: store}
	out.State = account15.State{Address: addr}
	return &out, nil
}

type state15 struct {
	account15.State
	store adt.Store
}

func (s *state15) PubkeyAddress() (address.Address, error) {
	return s.Address, nil
}

func (s *state15) GetState() interface{} {
	return &s.State
}

func (s *state15) ActorKey() string {
	return manifest.AccountKey
}

func (s *state15) ActorVersion() actorstypes.Version {
	return actorstypes.Version15
}

func (s *state15) Code() cid.Cid {
	code, ok := actors.GetActorCodeID(s.ActorVersion(), s.ActorKey())
	if !ok {
		panic(fmt.Errorf("didn't find actor %v code id for actor version %d", s.ActorKey(), s.ActorVersion()))
	}

	return code
}
