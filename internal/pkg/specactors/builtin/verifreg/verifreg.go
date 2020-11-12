package verifreg

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/internal/pkg/types"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/cbor"
	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"

	"github.com/filecoin-project/venus/internal/pkg/specactors/adt"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin"
)

func init() {
	builtin.RegisterActorState(builtin0.VerifiedRegistryActorCodeID, func(store adt.Store, root cid.Cid) (cbor.Marshaler, error) {
		return load0(store, root)
	})
	builtin.RegisterActorState(builtin2.VerifiedRegistryActorCodeID, func(store adt.Store, root cid.Cid) (cbor.Marshaler, error) {
		return load2(store, root)
	})
}

var (
	Address = builtin2.VerifiedRegistryActorAddr
	Methods = builtin2.MethodsVerifiedRegistry
)

func Load(store adt.Store, act *types.Actor) (State, error) {
	switch act.Code.Cid {
	case builtin0.VerifiedRegistryActorCodeID:
		return load0(store, act.Head.Cid)
	case builtin2.VerifiedRegistryActorCodeID:
		return load2(store, act.Head.Cid)
	}
	return nil, xerrors.Errorf("unknown actor code %s", act.Code)
}

type State interface {
	cbor.Marshaler

	RootKey() (address.Address, error)
	VerifiedClientDataCap(address.Address) (bool, abi.StoragePower, error)
	VerifierDataCap(address.Address) (bool, abi.StoragePower, error)
	ForEachVerifier(func(addr address.Address, dcap abi.StoragePower) error) error
	ForEachClient(func(addr address.Address, dcap abi.StoragePower) error) error
}
