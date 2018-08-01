package abi

import (
	"fmt"
	"math/big"
	"reflect"

	cbor "gx/ipfs/QmSyK1ZiAP98YvnxsTfQpb669V2xeTHRbG4Y6fgKS3vVSd/go-ipld-cbor"
	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

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
	// AttoFIL is a *types.AttoFIL
	AttoFIL
	// BytesAmount is a *types.BytesAmount
	BytesAmount
	// ChannelID is a *types.ChannelID
	ChannelID
	// BlockHeight is a *types.BlockHeight
	BlockHeight
	// Integer is a *big.Int
	Integer
	// Bytes is a []byte
	Bytes
	// String is a string
	String
	// UintArray is an array of uint64
	UintArray
	// PeerID is a libp2p peer ID
	PeerID
)

func (t Type) String() string {
	switch t {
	case Invalid:
		return "<invalid>"
	case Address:
		return "types.Address"
	case AttoFIL:
		return "*types.AttoFIL"
	case BytesAmount:
		return "*types.BytesAmount"
	case ChannelID:
		return "*types.ChannelID"
	case BlockHeight:
		return "*types.BlockHeight"
	case Integer:
		return "*big.Int"
	case Bytes:
		return "[]byte"
	case String:
		return "string"
	case UintArray:
		return "[]uint64"
	case PeerID:
		return "peer.ID"
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
	case AttoFIL:
		return av.Val.(*types.AttoFIL).String()
	case BytesAmount:
		return av.Val.(*types.BytesAmount).String()
	case ChannelID:
		return av.Val.(*types.ChannelID).String()
	case BlockHeight:
		return av.Val.(*types.BlockHeight).String()
	case Integer:
		return av.Val.(*big.Int).String()
	case Bytes:
		return string(av.Val.([]byte))
	case String:
		return av.Val.(string)
	case UintArray:
		return fmt.Sprint(av.Val.([]uint64))
	case PeerID:
		return av.Val.(peer.ID).String()
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
	case AttoFIL:
		ba, ok := av.Val.(*types.AttoFIL)
		if !ok {
			return nil, &typeError{types.AttoFIL{}, av.Val}
		}
		return ba.Bytes(), nil
	case BytesAmount:
		ba, ok := av.Val.(*types.BytesAmount)
		if !ok {
			return nil, &typeError{types.BytesAmount{}, av.Val}
		}
		return ba.Bytes(), nil
	case ChannelID:
		ba, ok := av.Val.(*types.ChannelID)
		if !ok {
			return nil, &typeError{types.ChannelID{}, av.Val}
		}
		return ba.Bytes(), nil
	case BlockHeight:
		ba, ok := av.Val.(*types.BlockHeight)
		if !ok {
			return nil, &typeError{types.BlockHeight{}, av.Val}
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
	case PeerID:
		pid, ok := av.Val.(peer.ID)
		if !ok {
			return nil, &typeError{peer.ID(""), av.Val}
		}

		return []byte(pid), nil
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
		case *types.AttoFIL:
			out = append(out, &Value{Type: AttoFIL, Val: v})
		case *types.BytesAmount:
			out = append(out, &Value{Type: BytesAmount, Val: v})
		case *types.ChannelID:
			out = append(out, &Value{Type: ChannelID, Val: v})
		case *types.BlockHeight:
			out = append(out, &Value{Type: BlockHeight, Val: v})
		case *big.Int:
			out = append(out, &Value{Type: Integer, Val: v})
		case []byte:
			out = append(out, &Value{Type: Bytes, Val: v})
		case string:
			out = append(out, &Value{Type: String, Val: v})
		case []uint64:
			out = append(out, &Value{Type: UintArray, Val: v})
		case peer.ID:
			out = append(out, &Value{Type: PeerID, Val: v})
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
	case AttoFIL:
		return &Value{
			Type: t,
			Val:  types.NewAttoFILFromBytes(data),
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
	case ChannelID:
		return &Value{
			Type: t,
			Val:  types.NewChannelIDFromBytes(data),
		}, nil
	case BlockHeight:
		return &Value{
			Type: t,
			Val:  types.NewBlockHeightFromBytes(data),
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
	case UintArray:
		var arr []uint64
		if err := cbor.DecodeInto(data, &arr); err != nil {
			return nil, err
		}
		return &Value{
			Type: t,
			Val:  arr,
		}, nil
	case PeerID:
		id, err := peer.IDFromBytes(data)
		if err != nil {
			return nil, err
		}

		return &Value{
			Type: t,
			Val:  id,
		}, nil
	case Invalid:
		return nil, ErrInvalidType
	default:
		return nil, fmt.Errorf("unrecognized Type: %d", t)
	}
}

var typeTable = map[Type]reflect.Type{
	Address:     reflect.TypeOf(types.Address{}),
	AttoFIL:     reflect.TypeOf(&types.AttoFIL{}),
	Bytes:       reflect.TypeOf([]byte{}),
	BytesAmount: reflect.TypeOf(&types.BytesAmount{}),
	ChannelID:   reflect.TypeOf(&types.ChannelID{}),
	BlockHeight: reflect.TypeOf(&types.BlockHeight{}),
	Integer:     reflect.TypeOf(&big.Int{}),
	String:      reflect.TypeOf(string("")),
	UintArray:   reflect.TypeOf([]uint64{}),
	PeerID:      reflect.TypeOf(peer.ID("")),
}

// TypeMatches returns whether or not 'val' is the go type expected for the given ABI type
func TypeMatches(t Type, val reflect.Type) bool {
	rt, ok := typeTable[t]
	if !ok {
		return false
	}
	return rt == val
}
