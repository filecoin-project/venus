package typeutil

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

type ABool bool
type AInt int
type AInt8 int8
type AInt16 int16
type AInt32 int32
type AInt64 int64
type AUInt uint
type AUInt8 uint8
type AUInt16 uint16
type AUInt32 uint32
type AUInt64 uint64
type AFloat32 float32
type AFloat64 float64
type AComplet64 complex64
type AComplet128 complex128
type AString string
type AUintptr uintptr
type AUnsafePointer unsafe.Pointer

type BBool bool
type BInt int
type BInt8 int8
type BInt16 int16
type BInt32 int32
type BInt64 int64
type BUInt uint
type BUInt8 uint8
type BUInt16 uint16
type BUInt32 uint32
type BUInt64 uint64
type BFloat32 float32
type BFloat64 float64
type BComplet64 complex64
type BComplet128 complex128
type BString string
type BUintptr uintptr
type BUnsafePointer unsafe.Pointer

func TestSimilarSimple(t *testing.T) {
	alist := []interface{}{
		new(ABool),
		new(AInt),
		new(AInt8),
		new(AInt16),
		new(AInt32),
		new(AInt64),
		new(AUInt),
		new(AUInt8),
		new(AUInt16),
		new(AUInt32),
		new(AUInt64),
		new(AFloat32),
		new(AFloat64),
		new(AComplet64),
		new(AComplet128),
		new(AString),
		new(AUintptr),
		new(AUnsafePointer),
	}

	blist := []interface{}{
		new(BBool),
		new(BInt),
		new(BInt8),
		new(BInt16),
		new(BInt32),
		new(BInt64),
		new(BUInt),
		new(BUInt8),
		new(BUInt16),
		new(BUInt32),
		new(BUInt64),
		new(BFloat32),
		new(BFloat64),
		new(BComplet64),
		new(BComplet128),
		new(BString),
		new(BUintptr),
		new(BUnsafePointer),
	}

	require.Equal(t, len(alist), len(blist), "values of newed types")

	for i := range alist {
		aval, bval := alist[i], blist[i]

		yes, reason := Similar(aval, bval, 0, 0)
		require.Truef(t, yes, "similar result for %T <-> %T: %s", aval, bval, reason)

		ratyp, rbtyp := reflect.TypeOf(aval), reflect.TypeOf(bval)
		require.Truef(t, ratyp != rbtyp, "not same type: %s vs %s", ratyp, rbtyp)

		yes, reason = Similar(ratyp, rbtyp, 0, 0)
		require.Truef(t, yes, "similar result for reflect.Type of %s <-> %s: %s", ratyp, rbtyp, reason)

		yes, reason = Similar(ratyp.Elem(), rbtyp.Elem(), 0, 0)
		require.Truef(t, yes, "similar result for reflect.Type of %s <-> %s: %s", ratyp.Elem(), rbtyp.Elem(), reason)
	}
}

type similarCase struct {
	val       interface{}
	codecFlag CodecFlag
	ordered   Ordered
	reasons   []error
}

func similarTest(t *testing.T, origin interface{}, cases []similarCase, checkIndirect bool) {
	valOrigin := reflect.ValueOf(origin)
	typOrigin := valOrigin.Type()
	indirect := checkIndirect && reflect.Indirect(valOrigin).Type() != typOrigin

	for i := range cases {
		expectedYes := len(cases[i].reasons) == 0

		typCase := reflect.TypeOf(cases[i].val)
		require.NotEqual(t, typOrigin, typCase, "types should be different")
		require.Equal(t, typOrigin.Kind(), typCase.Kind(), "kind should not be different")

		yes, reason := Similar(typOrigin, typCase, cases[i].codecFlag, cases[i].ordered)

		require.Equalf(t, expectedYes, yes, "#%d similar result for %s <> %s", i, typOrigin, typCase)
		if expectedYes {
			require.Nil(t, reason, "reason should be nil")
		} else {
			require.NotNil(t, reason, "reason should not be nil")
			for ei := range cases[i].reasons {
				ce := cases[i].reasons[ei]
				require.Truef(t, errors.Is(reason, ce), "for case #%d %s <> %s, reason should contains base %s, actual: %s", i, typOrigin, typCase, ce, reason)
			}
		}

		if indirect {
			require.Equal(t, typOrigin.Elem().Kind(), typCase.Elem().Kind(), "kind of indirect type should not be different")

			yes, reason = Similar(typOrigin.Elem(), typCase.Elem(), cases[i].codecFlag, cases[i].ordered)

			require.Equalf(t, expectedYes, yes, "#%d similar result for %s <> %s", i, typOrigin.Elem(), typCase.Elem())
			if expectedYes {
				require.Nil(t, reason, "reason should be nil")
			} else {
				require.NotNil(t, reason, "reason should not be nil")
				for ei := range cases[i].reasons {
					ce := cases[i].reasons[ei]
					require.Truef(t, errors.Is(reason, ce), "for case #%d %s <> %s, reason should contains base %s, actual: %s", i, typOrigin.Elem(), typCase.Elem(), ce, reason)
				}
			}
		}
	}
}

func TestArray(t *testing.T) {
	type origin [2]int
	type case1 [2]uint
	type case2 [3]int

	type case3 [2]AInt
	type case4 [3]AInt

	cases := []similarCase{
		{
			val:     new(case1),
			reasons: []error{ReasonArrayElement},
		},
		{
			val:     new(case2),
			reasons: []error{ReasonArrayLength},
		},
		{
			val: new(case3),
		},
		{
			val:     new(case4),
			reasons: []error{ReasonArrayLength},
		},
	}

	similarTest(t, new(origin), cases, true)
}

func TestMap(t *testing.T) {
	type origin map[string]int

	type case1 map[int]int
	type case2 map[string]string

	type case3 map[string]AInt
	type case4 map[AString]AInt

	cases := []similarCase{
		{
			val:     new(case1),
			reasons: []error{ReasonMapKey},
		},
		{
			val:     new(case2),
			reasons: []error{ReasonMapValue},
		},
		{
			val: new(case3),
		},
		{
			val: new(case4),
		},
	}

	similarTest(t, new(origin), cases, true)
}

func TestSlice(t *testing.T) {
	type origin []int

	type case1 []uint
	type case2 []string

	type case3 []AInt

	cases := []similarCase{
		{
			val:     new(case1),
			reasons: []error{ReasonSliceElement},
		},
		{
			val:     new(case2),
			reasons: []error{ReasonSliceElement},
		},
		{
			val: new(case3),
		},
	}

	similarTest(t, new(origin), cases, true)
}

func TestChan(t *testing.T) {
	type origin chan int

	type case1 chan uint
	type case2 <-chan int
	type case3 chan<- int

	type case4 chan AInt

	cases := []similarCase{
		{
			val:     new(case1),
			reasons: []error{ReasonChanElement},
		},
		{
			val:     new(case2),
			reasons: []error{ReasonChanDir},
		},
		{
			val:     new(case3),
			reasons: []error{ReasonChanDir},
		},
		{
			val: new(case4),
		},
	}

	similarTest(t, new(origin), cases, true)
}

func TestStruct(t *testing.T) {
	type origin struct {
		A uint
		B int
	}

	type case1 struct {
		C uint
		B int
	}

	type case2 struct {
		B int
		A uint
	}

	type case3 struct {
		B AInt
		A AUInt
	}

	type case4 struct {
		A AUInt
		B AInt
		a bool // nolint
	}

	type case5 struct {
		A AUInt
		b AInt // nolint
	}

	type case6 struct {
		A uint
		B uint
	}

	cases := []similarCase{
		{
			val:     new(case1),
			ordered: StructFieldsOrdered,
			reasons: []error{ReasonStructField, ReasonExportedFieldName},
		},
		{
			val:     new(case1),
			ordered: 0,
			reasons: []error{ReasonStructField, ReasonExportedFieldNotFound},
		},
		{
			val:     new(case2),
			ordered: StructFieldsOrdered,
			reasons: []error{ReasonStructField, ReasonExportedFieldName},
		},
		{
			val:     new(case2),
			ordered: 0,
		},
		{
			val:     new(case3),
			ordered: StructFieldsOrdered,
			reasons: []error{ReasonStructField, ReasonExportedFieldName},
		},
		{
			val:     new(case3),
			ordered: 0,
		},
		{
			val:     new(case4),
			ordered: 0,
		},
		{
			val:     new(case4),
			ordered: StructFieldsOrdered,
		},
		{
			val:     new(case5),
			ordered: 0,
			reasons: []error{ReasonStructField, ReasonExportedFieldsCount},
		},
		{
			val:     new(case5),
			ordered: StructFieldsOrdered,
			reasons: []error{ReasonStructField, ReasonExportedFieldsCount},
		},
		{
			val:     new(case6),
			ordered: 0,
			reasons: []error{ReasonStructField, ReasonExportedFieldType},
		},
		{
			val:     new(case6),
			ordered: StructFieldsOrdered,
			reasons: []error{ReasonStructField, ReasonExportedFieldType},
		},
	}

	similarTest(t, new(origin), cases, true)
}

func TestInterface(t *testing.T) {
	type origin interface {
		Read(context.Context) (int, error)
		Write(context.Context, []byte) (int, error)
		Close(context.Context) error
	}

	type case1 interface {
		Write(context.Context, []byte) (int, error)
		Read(context.Context) (int, error)
		Close(context.Context) error
	}

	type case2 interface {
		Read1(context.Context) (int, error)
		Write(context.Context, []byte) (int, error)
		Close(context.Context) error
	}

	type case3 interface {
		Read(context.Context, []byte) (int, error)
		Write(context.Context, []byte) (int, error)
		Close(context.Context) error
	}

	type case4 interface {
		Read(context.Context) error
		Write(context.Context, []byte) (int, error)
		Close(context.Context) error
	}

	type Bytes []byte
	type case5 interface {
		Read(context.Context) (AInt, error)
		Write(context.Context, Bytes) (AInt, error)
		Close(context.Context) error
	}

	type case6 interface {
		Read(context.Context) (AInt, error)
		Write(context.Context, Bytes) (AInt, error)
	}

	type case7 interface {
		Read(context.Context) (AUInt, error)
		Write(context.Context, Bytes) (AInt, error)
		Close(context.Context) error
	}

	type case8 interface {
		Read(context.Context) (AInt, error)
		Write(context.Context, string) (AInt, error)
		Close(context.Context) error
	}

	type case9 interface {
		Read(context.Context) (AInt, error)
		Write(context.Context, Bytes) (AInt, error)
		Close(context.Context) error
		read(context.Context)
	}

	cases := []similarCase{
		{
			val: new(case1),
		},
		{
			val:     new(case2),
			reasons: []error{ReasonExportedMethodName},
		},
		{
			val:     new(case3),
			reasons: []error{ReasonExportedMethodType, ReasonFuncInNum},
		},
		{
			val:     new(case4),
			reasons: []error{ReasonExportedMethodType, ReasonFuncOutNum},
		},
		{
			val: new(case5),
		},
		{
			val:     new(case6),
			reasons: []error{ReasonExportedMethodsCount},
		},
		{
			val:     new(case7),
			reasons: []error{ReasonExportedMethodType, ReasonFuncOutType},
		},
		{
			val:     new(case8),
			reasons: []error{ReasonExportedMethodType, ReasonFuncInType},
		},
		{
			val: new(case9),
		},
	}

	similarTest(t, new(origin), cases, true)
}

type codecInt int

func (ci codecInt) MarshalBinary() ([]byte, error) { // nolint
	panic("not impl")
}

func (ci *codecInt) UnmarshalBinary([]byte) error { // nolint
	panic("not impl")
}

func (ci codecInt) MarshalText() ([]byte, error) { // nolint
	panic("not impl")
}

func (ci *codecInt) UnmarshalText([]byte) error { // nolint
	panic("not impl")
}

func (ci codecInt) MarshalJSON() ([]byte, error) { // nolint
	panic("not impl")
}

func (ci *codecInt) UnmarshalJSON([]byte) error { // nolint
	panic("not impl")
}

func (ci codecInt) MarshalCBOR(w io.Writer) error { // nolint
	panic("not impl")
}

func (ci *codecInt) UnmarshalCBOR(r io.Reader) error { // nolint
	panic("not impl")
}

type halfCodecInt int

func (ci halfCodecInt) MarshalBinary() ([]byte, error) { // nolint
	panic("not impl")
}

func (ci halfCodecInt) MarshalText() ([]byte, error) { // nolint
	panic("not impl")
}

func (ci halfCodecInt) MarshalJSON() ([]byte, error) { // nolint
	panic("not impl")
}

func (ci halfCodecInt) MarshalCBOR(w io.Writer) error { // nolint
	panic("not impl")
}

func TestCodec(t *testing.T) {
	cases := []similarCase{
		{
			val:       new(AInt),
			codecFlag: 0,
		},
		{
			val:       new(AInt),
			codecFlag: BinaryCodec,
			reasons:   []error{ReasonCodecMarshalerImplementations},
		},
		{
			val:       new(AInt),
			codecFlag: TextCodec,
			reasons:   []error{ReasonCodecMarshalerImplementations},
		},
		{
			val:       new(AInt),
			codecFlag: JSONCodec,
			reasons:   []error{ReasonCodecMarshalerImplementations},
		},
		{
			val:       new(AInt),
			codecFlag: CborCodec,
			reasons:   []error{ReasonCodecMarshalerImplementations},
		},
		{
			val:       new(codecInt),
			codecFlag: 0,
		},
		{
			val:       new(codecInt),
			codecFlag: BinaryCodec,
			reasons:   []error{ReasonCodecUnmarshalerImplementations},
		},
		{
			val:       new(codecInt),
			codecFlag: TextCodec,
			reasons:   []error{ReasonCodecUnmarshalerImplementations},
		},
		{
			val:       new(codecInt),
			codecFlag: JSONCodec,
			reasons:   []error{ReasonCodecUnmarshalerImplementations},
		},
		{
			val:       new(codecInt),
			codecFlag: CborCodec,
			reasons:   []error{ReasonCodecUnmarshalerImplementations},
		},
	}

	similarTest(t, new(halfCodecInt), cases, false)
}
