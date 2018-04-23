package abi

import (
	"fmt"
	"math/big"
	"reflect"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/types"
)

// ErrInvalidType is returned when processing a zero valued 'Type' (aka Invalid)
var ErrInvalidType = fmt.Errorf("invalid type")

// Type represents a type that can be passed through the filecoin ABI
type Type uint64

const (
	// Invalid is the default value for 'Type' and represents an errorneously set type.
	Invalid = Type(iota)
	// Address is a types.Address
	Address
	// TokenAmount is a *types.TokenAmount
	TokenAmount
	// BytesAmount is a *types.BytesAmount
	BytesAmount
	// Integer is a *big.Int
	Integer
	// Bytes is a []byte
	Bytes
	// String is a string
	String
	// UintArray is an array of uint64
	UintArray
)

func (t Type) String() string {
	switch t {
	case Invalid:
		return "<invalid>"
	case Address:
		return "types.Address"
	case TokenAmount:
		return "*types.TokenAmount"
	case BytesAmount:
		return "*types.BytesAmount"
	case Integer:
		return "*big.Int"
	case Bytes:
		return "[]byte"
	case String:
		return "string"
	case UintArray:
		return "[]uint64"
	default:
		return "<unknown type>"
	}
}

// Value pairs a go value with its ABI type
type Value struct {
	Type Type
	Val  interface{}
}

func (av *Value) String() string {
	switch av.Type {
	case Invalid:
		return "<invalid>"
	case Address:
		return av.Val.(types.Address).String()
	case TokenAmount:
		return av.Val.(*types.TokenAmount).String()
	case BytesAmount:
		return av.Val.(*types.BytesAmount).String()
	case Integer:
		return av.Val.(*big.Int).String()
	case Bytes:
		return string(av.Val.([]byte))
	case String:
		return av.Val.(string)
	case UintArray:
		return fmt.Sprint(av.Val.([]uint64))
	default:
		return "<unknown type>"
	}
}

type typeError struct {
	exp interface{}
	got interface{}
}

func (ate typeError) Error() string {
	return fmt.Sprintf("expected type %T, got %T", ate.exp, ate.got)
}

// Serialize serializes the value into raw bytes. Only works on valid supported types.
func (av *Value) Serialize() ([]byte, error) {
	switch av.Type {
	case Invalid:
		return nil, ErrInvalidType
	case Address:
		addr, ok := av.Val.(types.Address)
		if !ok {
			return nil, &typeError{types.Address{}, av.Val}
		}
		return addr.Bytes(), nil
	case TokenAmount:
		ba, ok := av.Val.(*types.TokenAmount)
		if !ok {
			return nil, &typeError{types.TokenAmount{}, av.Val}
		}
		return ba.Bytes(), nil
	case BytesAmount:
		ba, ok := av.Val.(*types.BytesAmount)
		if !ok {
			return nil, &typeError{types.BytesAmount{}, av.Val}
		}
		return ba.Bytes(), nil
	case Integer:
		intgr, ok := av.Val.(*big.Int)
		if !ok {
			return nil, &typeError{&big.Int{}, av.Val}
		}
		return intgr.Bytes(), nil
	case Bytes:
		b, ok := av.Val.([]byte)
		if !ok {
			return nil, &typeError{[]byte{}, av.Val}
		}
		return b, nil
	case String:
		s, ok := av.Val.(string)
		if !ok {
			return nil, &typeError{"", av.Val}
		}

		return []byte(s), nil
	case UintArray:
		arr, ok := av.Val.([]uint64)
		if !ok {
			return nil, &typeError{[]uint64{}, av.Val}
		}

		return cbor.DumpObject(arr)
	default:
		return nil, fmt.Errorf("unrecognized Type: %d", av.Type)
	}
}

// ToValues converts from a slice of go abi-compatible values to abi values.
// empty slices are normalized to nil
func ToValues(i []interface{}) ([]*Value, error) {
	if len(i) == 0 {
		return nil, nil
	}

	out := make([]*Value, 0, len(i))
	for _, v := range i {
		switch v := v.(type) {
		case types.Address:
			out = append(out, &Value{Type: Address, Val: v})
		case *types.TokenAmount:
			out = append(out, &Value{Type: TokenAmount, Val: v})
		case *types.BytesAmount:
			out = append(out, &Value{Type: BytesAmount, Val: v})
		case *big.Int:
			out = append(out, &Value{Type: Integer, Val: v})
		case []byte:
			out = append(out, &Value{Type: Bytes, Val: v})
		case string:
			out = append(out, &Value{Type: String, Val: v})
		case []uint64:
			out = append(out, &Value{Type: UintArray, Val: v})
		default:
			return nil, fmt.Errorf("unsupported type: %T", v)
		}
	}
	return out, nil
}

// FromValues converts from a slice of abi values to the go type representation
// of them. empty slices are normalized to nil
func FromValues(vals []*Value) []interface{} {
	if len(vals) == 0 {
		return nil
	}

	out := make([]interface{}, 0, len(vals))
	for _, v := range vals {
		out = append(out, v.Val)
	}
	return out
}

// Deserialize converts the given bytes to the requested type and returns an
// ABI Value for it.
func Deserialize(data []byte, t Type) (*Value, error) {
	switch t {
	case Address:
		addr, err := types.NewAddressFromBytes(data)
		if err != nil {
			return nil, err
		}

		return &Value{
			Type: t,
			Val:  addr,
		}, nil
	case Bytes:
		return &Value{
			Type: t,
			Val:  data,
		}, nil
	case BytesAmount:
		return &Value{
			Type: t,
			Val:  types.NewBytesAmountFromBytes(data),
		}, nil
	case Integer:
		return &Value{
			Type: t,
			Val:  big.NewInt(0).SetBytes(data),
		}, nil
	case String:
		return &Value{
			Type: t,
			Val:  string(data),
		}, nil
	case TokenAmount:
		return &Value{
			Type: t,
			Val:  types.NewTokenAmountFromBytes(data),
		}, nil
	case UintArray:
		var arr []uint64
		if err := cbor.DecodeInto(data, &arr); err != nil {
			return nil, err
		}
		return &Value{
			Type: t,
			Val:  arr,
		}, nil
	case Invalid:
		return nil, ErrInvalidType
	default:
		return nil, fmt.Errorf("unrecognized Type: %d", t)
	}
}

var typeTable = map[Type]reflect.Type{
	Address:     reflect.TypeOf(types.Address{}),
	Bytes:       reflect.TypeOf([]byte{}),
	BytesAmount: reflect.TypeOf(&types.BytesAmount{}),
	Integer:     reflect.TypeOf(&big.Int{}),
	String:      reflect.TypeOf(string("")),
	TokenAmount: reflect.TypeOf(&types.TokenAmount{}),
	UintArray:   reflect.TypeOf([]uint64{}),
}

// TypeMatches returns whether or not 'val' is the go type expected for the given ABI type
func TypeMatches(t Type, val reflect.Type) bool {
	rt, ok := typeTable[t]
	if !ok {
		return false
	}
	return rt == val
}
