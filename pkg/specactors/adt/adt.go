package adt

import (
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/cbor"
)

type Map interface {
	Root() (cid.Cid, error)

	Put(k abi.Keyer, v cbor.Marshaler) error
	Get(k abi.Keyer, v cbor.Unmarshaler) (bool, error)
	Delete(k abi.Keyer) error

	ForEach(v cbor.Unmarshaler, fn func(key string) error) error
}

//func AsMap(store Store, root cid.Cid, version specactors.Version) (Map, error) {
//	switch version {
//	case specactors.Version0:
//		return adt0.AsMap(store, root)
//	case specactors.Version2:
//		return adt2.AsMap(store, root)
//	case specactors.Version3:
//		return adt3.AsMap(store, root, builtin3.DefaultHamtBitwidth)
//	}
//	return nil, xerrors.Errorf("unknown network version: %d", version)
//}
//
//func NewMap(store Store, version specactors.Version) (Map, error) {
//	switch version {
//	case specactors.Version0:
//		return adt0.MakeEmptyMap(store), nil
//	case specactors.Version2:
//		return adt2.MakeEmptyMap(store), nil
//	case specactors.Version3:
//		return adt3.MakeEmptyMap(store, builtin3.DefaultHamtBitwidth), nil
//	}
//	return nil, xerrors.Errorf("unknown network version: %d", version)
//}

type Array interface {
	Root() (cid.Cid, error)

	Set(idx uint64, v cbor.Marshaler) error
	Get(idx uint64, v cbor.Unmarshaler) (bool, error)
	Delete(idx uint64) error
	Length() uint64

	ForEach(v cbor.Unmarshaler, fn func(idx int64) error) error
}
