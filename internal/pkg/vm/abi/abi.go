package abi

import (
	"fmt"
	"math/big"
	"reflect"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-leb128"
)

// ErrInvalidType is returned when processing a zero valued 'Type' (aka Invalid)
var ErrInvalidType = fmt.Errorf("invalid type")

// Type represents a type that can be passed through the filecoin ABI
type Type uint64

const (
	// Invalid is the default value for 'Type' and represents an erroneously set type.
	Invalid = Type(iota)
	// Address is a address.Address
	Address
	// AttoFIL is a types.AttoFIL
	AttoFIL
	// BytesAmount is a *types.BytesAmount
	BytesAmount
	// ChannelID is a *types.ChannelID
	ChannelID
	// Cid is a cid.Cid
	Cid
	// BlockHeight is a *types.BlockHeight
	BlockHeight
	// Integer is a *big.Int
	Integer
	// Bytes is a []byte
	Bytes
	// String is a string
	String
	// UintArray is an array of types.Uint64
	UintArray
	// PeerID is a libp2p peer ID
	PeerID
	// SectorID is a uint64
	SectorID
	// CommitmentsMap is a map of stringified sector id (uint64) to commitments
	CommitmentsMap
	// Boolean is a bool
	Boolean
	// ProofsMode is an enumeration of possible modes of proof operation
	ProofsMode
	// PoRepProof is a dynamic length array of the PoRep proof-bytes
	PoRepProof
	// PoStProof is a dynamic length array of the PoSt proof-bytes
	PoStProof
	// Predicate is subset of a message used to ask an actor about a condition
	Predicate
	// Parameters is a slice of individually encodable parameters
	Parameters
	// IntSet is a set of uint64
	IntSet
	// MinerPoStStates is a *map[string]uint64, where string is address.Address.String()
	// and uint8 is a miner PoStState
	MinerPoStStates
	// FaultSet is the faults generated during PoSt generation
	FaultSet
	// PowerReport is a tuple of *types.ByteAmount
	PowerReport
	// FaultReport is a triple of integers
	FaultReport
	// StorageDealProposals is a slice of deals
	StorageDealProposals
	// SectorPreCommitInfo are parameters to SectorPreCommit
	SectorPreCommitInfo
	// SectorProveCommitInfo are parameters to SectorProveCommit
	SectorProveCommitInfo
)

func (t Type) String() string {
	switch t {
	case Invalid:
		return "<invalid>"
	case Address:
		return "address.Address"
	case AttoFIL:
		return "types.AttoFIL"
	case BytesAmount:
		return "*types.BytesAmount"
	case ChannelID:
		return "*types.ChannelID"
	case Cid:
		return "cid.Cid"
	case BlockHeight:
		return "*types.BlockHeight"
	case Integer:
		return "*big.Int"
	case Bytes:
		return "[]byte"
	case String:
		return "string"
	case UintArray:
		return "[]types.Uint64"
	case PeerID:
		return "peer.ID"
	case SectorID:
		return "uint64"
	case CommitmentsMap:
		return "map[string]types.Commitments"
	case Boolean:
		return "bool"
	case ProofsMode:
		return "types.ProofsMode"
	case PoRepProof:
		return "types.PoRepProof"
	case PoStProof:
		return "types.PoStProof"
	case Predicate:
		return "*types.Predicate"
	case Parameters:
		return "[]interface{}"
	case IntSet:
		return "types.IntSet"
	case MinerPoStStates:
		return "*map[string]uint64"
	case FaultSet:
		return "types.FaultSet"
	case PowerReport:
		return "types.PowerReport"
	case FaultReport:
		return "types.FaultReport"
	case StorageDealProposals:
		return "[]types.StorageDealProposal"
	case SectorPreCommitInfo:
		return "types.SectorPreCommitInfo"
	case SectorProveCommitInfo:
		return "types.SectorProveCommitInfo"
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
		return av.Val.(address.Address).String()
	case AttoFIL:
		return av.Val.(types.AttoFIL).String()
	case BytesAmount:
		return av.Val.(*types.BytesAmount).String()
	case ChannelID:
		return av.Val.(*types.ChannelID).String()
	case Cid:
		return av.Val.(cid.Cid).String()
	case BlockHeight:
		return av.Val.(*types.BlockHeight).String()
	case Integer:
		return av.Val.(*big.Int).String()
	case Bytes:
		return string(av.Val.([]byte))
	case String:
		return av.Val.(string)
	case UintArray:
		return fmt.Sprint(av.Val.([]types.Uint64))
	case PeerID:
		return av.Val.(peer.ID).String()
	case SectorID:
		return fmt.Sprint(av.Val.(uint64))
	case CommitmentsMap:
		return fmt.Sprint(av.Val.(map[string]types.Commitments))
	case Boolean:
		return fmt.Sprint(av.Val.(bool))
	case ProofsMode:
		return fmt.Sprint(av.Val.(types.ProofsMode))
	case PoRepProof:
		return fmt.Sprint(av.Val.(types.PoRepProof))
	case PoStProof:
		return fmt.Sprint(av.Val.(types.PoStProof))
	case Predicate:
		return fmt.Sprint(av.Val.(*types.Predicate))
	case Parameters:
		return fmt.Sprint(av.Val.([]interface{}))
	case IntSet:
		return av.Val.(types.IntSet).String()
	case MinerPoStStates:
		return fmt.Sprint(av.Val.(*map[address.Address]uint8))
	case FaultSet:
		return av.Val.(types.FaultSet).String()
	case PowerReport:
		return fmt.Sprint(av.Val.(types.PowerReport))
	case FaultReport:
		return fmt.Sprint(av.Val.(types.FaultReport))
	case StorageDealProposals:
		return fmt.Sprint(av.Val.([]types.StorageDealProposal))
	case SectorPreCommitInfo:
		return fmt.Sprint(av.Val.(types.SectorPreCommitInfo))
	case SectorProveCommitInfo:
		return fmt.Sprint(av.Val.(types.SectorProveCommitInfo))
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
		addr, ok := av.Val.(address.Address)
		if !ok {
			return nil, &typeError{address.Undef, av.Val}
		}
		return addr.Bytes(), nil
	case AttoFIL:
		ba, ok := av.Val.(types.AttoFIL)
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
	case Cid:
		c, ok := av.Val.(cid.Cid)
		if !ok {
			return nil, &typeError{cid.Cid{}, av.Val}
		}
		return c.Bytes(), nil
	case BlockHeight:
		ba, ok := av.Val.(*types.BlockHeight)
		if !ok {
			return nil, &typeError{types.BlockHeight{}, av.Val}
		}
		if ba == nil {
			return nil, nil
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
		arr, ok := av.Val.([]types.Uint64)
		if !ok {
			return nil, &typeError{[]types.Uint64{}, av.Val}
		}

		return encoding.Encode(arr)
	case PeerID:
		pid, ok := av.Val.(peer.ID)
		if !ok {
			return nil, &typeError{peer.ID(""), av.Val}
		}

		return []byte(pid), nil
	case SectorID:
		n, ok := av.Val.(uint64)
		if !ok {
			return nil, &typeError{0, av.Val}
		}

		return leb128.FromUInt64(n), nil
	case CommitmentsMap:
		m, ok := av.Val.(map[string]types.Commitments)
		if !ok {
			return nil, &typeError{map[string]types.Commitments{}, av.Val}
		}

		return encoding.Encode(m)
	case Boolean:
		v, ok := av.Val.(bool)
		if !ok {
			return nil, &typeError{false, av.Val}
		}

		var b byte
		if v {
			b = 1
		}

		return []byte{b}, nil
	case ProofsMode:
		v, ok := av.Val.(types.ProofsMode)
		if !ok {
			return nil, &typeError{types.TestProofsMode, av.Val}
		}

		return []byte{byte(v)}, nil
	case PoRepProof:
		b, ok := av.Val.(types.PoRepProof)
		if !ok {
			return nil, &typeError{types.PoRepProof{}, av.Val}
		}
		return b, nil
	case PoStProof:
		b, ok := av.Val.(types.PoStProof)
		if !ok {
			return nil, &typeError{types.PoStProof{}, av.Val}
		}
		return b, nil
	case Predicate:
		p, ok := av.Val.(*types.Predicate)
		if !ok {
			return nil, &typeError{&types.Predicate{}, av.Val}
		}

		return encoding.Encode(p)
	case Parameters:
		p, ok := av.Val.([]interface{})
		if !ok {
			return nil, &typeError{[]interface{}{}, av.Val}
		}

		return encoding.Encode(p)
	case IntSet:
		is, ok := av.Val.(types.IntSet)
		if !ok {
			return nil, &typeError{types.IntSet{}, av.Val}
		}
		return encoding.Encode(is)
	case MinerPoStStates:
		addrs, ok := av.Val.(*map[string]uint64)
		if !ok {
			return nil, &typeError{&map[string]uint64{}, av.Val}
		}
		return encoding.Encode(addrs)
	case FaultSet:
		fs, ok := av.Val.(types.FaultSet)
		if !ok {
			return nil, &typeError{types.FaultSet{}, av.Val}
		}
		return encoding.Encode(fs)
	case PowerReport:
		pr, ok := av.Val.(types.PowerReport)
		if !ok {
			return nil, &typeError{types.PowerReport{}, av.Val}
		}
		return encoding.Encode(pr)
	case FaultReport:
		fr, ok := av.Val.(types.FaultReport)
		if !ok {
			return nil, &typeError{types.FaultReport{}, av.Val}
		}
		return encoding.Encode(fr)
	case StorageDealProposals:
		sdp, ok := av.Val.([]types.StorageDealProposal)
		if !ok {
			return nil, &typeError{[]types.StorageDealProposal{}, av.Val}
		}
		return encoding.Encode(sdp)
	case SectorPreCommitInfo:
		spci, ok := av.Val.(types.SectorPreCommitInfo)
		if !ok {
			return nil, &typeError{types.SectorPreCommitInfo{}, av.Val}
		}
		return encoding.Encode(spci)
	case SectorProveCommitInfo:
		spci, ok := av.Val.(types.SectorProveCommitInfo)
		if !ok {
			return nil, &typeError{types.SectorProveCommitInfo{}, av.Val}
		}
		return encoding.Encode(spci)
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
		case address.Address:
			out = append(out, &Value{Type: Address, Val: v})
		case types.AttoFIL:
			out = append(out, &Value{Type: AttoFIL, Val: v})
		case *types.BytesAmount:
			out = append(out, &Value{Type: BytesAmount, Val: v})
		case *types.ChannelID:
			out = append(out, &Value{Type: ChannelID, Val: v})
		case cid.Cid:
			out = append(out, &Value{Type: Cid, Val: v})
		case *types.BlockHeight:
			out = append(out, &Value{Type: BlockHeight, Val: v})
		case *big.Int:
			out = append(out, &Value{Type: Integer, Val: v})
		case []byte:
			out = append(out, &Value{Type: Bytes, Val: v})
		case string:
			out = append(out, &Value{Type: String, Val: v})
		case []types.Uint64:
			out = append(out, &Value{Type: UintArray, Val: v})
		case peer.ID:
			out = append(out, &Value{Type: PeerID, Val: v})
		case uint64:
			out = append(out, &Value{Type: SectorID, Val: v})
		case map[string]types.Commitments:
			out = append(out, &Value{Type: CommitmentsMap, Val: v})
		case bool:
			out = append(out, &Value{Type: Boolean, Val: v})
		case types.ProofsMode:
			out = append(out, &Value{Type: ProofsMode, Val: v})
		case types.PoRepProof:
			out = append(out, &Value{Type: PoRepProof, Val: v})
		case types.PoStProof:
			out = append(out, &Value{Type: PoStProof, Val: v})
		case *types.Predicate:
			out = append(out, &Value{Type: Predicate, Val: v})
		case []interface{}:
			out = append(out, &Value{Type: Parameters, Val: v})
		case types.IntSet:
			out = append(out, &Value{Type: IntSet, Val: v})
		case *map[string]uint64:
			out = append(out, &Value{Type: MinerPoStStates, Val: v})
		case types.FaultSet:
			out = append(out, &Value{Type: FaultSet, Val: v})
		case types.PowerReport:
			out = append(out, &Value{Type: PowerReport, Val: v})
		case types.FaultReport:
			out = append(out, &Value{Type: FaultReport, Val: v})
		case []types.StorageDealProposal:
			out = append(out, &Value{Type: StorageDealProposals, Val: v})
		case types.SectorPreCommitInfo:
			out = append(out, &Value{Type: SectorPreCommitInfo, Val: v})
		case types.SectorProveCommitInfo:
			out = append(out, &Value{Type: SectorProveCommitInfo, Val: v})
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
		addr, err := address.NewFromBytes(data)
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
	case Cid:
		c, err := cid.Cast(data)
		if err != nil {
			return nil, err
		}
		return &Value{
			Type: t,
			Val:  c,
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
		var arr []types.Uint64
		if err := encoding.Decode(data, &arr); err != nil {
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
	case SectorID:
		return &Value{
			Type: t,
			Val:  leb128.ToUInt64(data),
		}, nil
	case CommitmentsMap:
		var m map[string]types.Commitments
		if err := encoding.Decode(data, &m); err != nil {
			return nil, err
		}
		return &Value{
			Type: t,
			Val:  m,
		}, nil
	case Boolean:
		var b bool
		if data[0] == 1 {
			b = true
		}
		return &Value{
			Type: t,
			Val:  b,
		}, nil
	case ProofsMode:
		return &Value{
			Type: t,
			Val:  types.ProofsMode(int(data[0])),
		}, nil
	case PoRepProof:
		return &Value{
			Type: t,
			Val:  append(types.PoRepProof{}, data[:]...),
		}, nil
	case PoStProof:
		return &Value{
			Type: t,
			Val:  append(types.PoStProof{}, data[:]...),
		}, nil
	case Predicate:
		var predicate *types.Predicate
		if err := encoding.Decode(data, &predicate); err != nil {
			return nil, err
		}
		return &Value{
			Type: t,
			Val:  predicate,
		}, nil
	case Parameters:
		var parameters []interface{}
		if err := encoding.Decode(data, &parameters); err != nil {
			return nil, err
		}
		return &Value{
			Type: t,
			Val:  parameters,
		}, nil
	case IntSet:
		var is types.IntSet
		if err := encoding.Decode(data, &is); err != nil {
			return nil, err
		}
		return &Value{
			Type: t,
			Val:  is,
		}, nil
	case MinerPoStStates:
		var lm *map[string]uint64
		if err := encoding.Decode(data, &lm); err != nil {
			return nil, err

		}
		return &Value{
			Type: t,
			Val:  lm,
		}, nil
	case FaultSet:
		fs := types.NewFaultSet([]uint64{})
		if err := encoding.Decode(data, &fs); err != nil {
			return nil, err
		}
		return &Value{
			Type: t,
			Val:  fs,
		}, nil
	case PowerReport:
		var pr types.PowerReport
		err := encoding.Decode(data, &pr)
		if err != nil {
			return nil, err
		}
		return &Value{
			Type: t,
			Val:  pr,
		}, nil
	case FaultReport:
		var fr types.FaultReport
		err := encoding.Decode(data, &fr)
		if err != nil {
			return nil, err
		}
		return &Value{
			Type: t,
			Val:  fr,
		}, nil
	case StorageDealProposals:
		var sdp []types.StorageDealProposal
		err := encoding.Decode(data, sdp)
		if err != nil {
			return nil, err
		}
		return &Value{
			Type: t,
			Val:  sdp,
		}, nil
	case SectorPreCommitInfo:
		var spci types.SectorPreCommitInfo
		err := encoding.Decode(data, &spci)
		if err != nil {
			return nil, err
		}
		return &Value{
			Type: t,
			Val:  spci,
		}, nil
	case SectorProveCommitInfo:
		var spci types.SectorProveCommitInfo
		err := encoding.Decode(data, &spci)
		if err != nil {
			return nil, err
		}
		return &Value{
			Type: t,
			Val:  spci,
		}, nil
	case Invalid:
		return nil, ErrInvalidType
	default:
		return nil, fmt.Errorf("unrecognized Type: %d", t)
	}
}

var typeTable = map[Type]reflect.Type{
	Address:               reflect.TypeOf(address.Address{}),
	AttoFIL:               reflect.TypeOf(types.AttoFIL{}),
	Bytes:                 reflect.TypeOf([]byte{}),
	BytesAmount:           reflect.TypeOf(&types.BytesAmount{}),
	ChannelID:             reflect.TypeOf(&types.ChannelID{}),
	Cid:                   reflect.TypeOf(cid.Cid{}),
	BlockHeight:           reflect.TypeOf(&types.BlockHeight{}),
	Integer:               reflect.TypeOf(&big.Int{}),
	String:                reflect.TypeOf(string("")),
	UintArray:             reflect.TypeOf([]types.Uint64{}),
	PeerID:                reflect.TypeOf(peer.ID("")),
	SectorID:              reflect.TypeOf(uint64(0)),
	CommitmentsMap:        reflect.TypeOf(map[string]types.Commitments{}),
	Boolean:               reflect.TypeOf(false),
	ProofsMode:            reflect.TypeOf(types.TestProofsMode),
	PoRepProof:            reflect.TypeOf(types.PoRepProof{}),
	PoStProof:             reflect.TypeOf(types.PoStProof{}),
	Predicate:             reflect.TypeOf(&types.Predicate{}),
	Parameters:            reflect.TypeOf([]interface{}{}),
	IntSet:                reflect.TypeOf(types.IntSet{}),
	MinerPoStStates:       reflect.TypeOf(&map[string]uint64{}),
	FaultSet:              reflect.TypeOf(types.FaultSet{}),
	PowerReport:           reflect.TypeOf(types.PowerReport{}),
	FaultReport:           reflect.TypeOf(types.FaultReport{}),
	StorageDealProposals:  reflect.TypeOf([]types.StorageDealProposal{}),
	SectorPreCommitInfo:   reflect.TypeOf(types.SectorPreCommitInfo{}),
	SectorProveCommitInfo: reflect.TypeOf(types.SectorProveCommitInfo{}),
}

// TypeMatches returns whether or not 'val' is the go type expected for the given ABI type
func TypeMatches(t Type, val reflect.Type) bool {
	rt, ok := typeTable[t]
	if !ok {
		return false
	}
	return rt == val
}
