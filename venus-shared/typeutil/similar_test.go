package typeutil

import (
	"context"
	"errors"
	"io"
	"math/bits"
	"reflect"
	"testing"
	"unsafe"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"

	"github.com/stretchr/testify/require"
)

func TestCodecList(t *testing.T) {
	tf.UnitTest(t)
	zeroes := bits.TrailingZeros(uint(_codecLimit))
	require.Equalf(t, zeroes, len(codecs), "codec count not match, %d != %d", zeroes, len(codecs))

	for ci := range codecs {
		czeroes := bits.TrailingZeros(uint(codecs[ci].flag))
		require.Equalf(t, czeroes, ci, "#%d codec's flag is not matched", ci)
	}
}

type (
	ABool          bool
	AInt           int
	AInt8          int8
	AInt16         int16
	AInt32         int32
	AInt64         int64
	AUInt          uint
	AUInt8         uint8
	AUInt16        uint16
	AUInt32        uint32
	AUInt64        uint64
	AFloat32       float32
	AFloat64       float64
	AComplet64     complex64
	AComplet128    complex128
	AString        string
	AUintptr       uintptr
	AUnsafePointer unsafe.Pointer
)

type (
	BBool          bool
	BInt           int
	BInt8          int8
	BInt16         int16
	BInt32         int32
	BInt64         int64
	BUInt          uint
	BUInt8         uint8
	BUInt16        uint16
	BUInt32        uint32
	BUInt64        uint64
	BFloat32       float32
	BFloat64       float64
	BComplet64     complex64
	BComplet128    complex128
	BString        string
	BUintptr       uintptr
	BUnsafePointer unsafe.Pointer
)

func TestSimilarSimple(t *testing.T) {
	tf.UnitTest(t)
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
	smode     SimilarMode
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

		yes, reason := Similar(typOrigin, typCase, cases[i].codecFlag, cases[i].smode)

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

			yes, reason = Similar(typOrigin.Elem(), typCase.Elem(), cases[i].codecFlag, cases[i].smode)

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
	tf.UnitTest(t)
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
	tf.UnitTest(t)
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
	tf.UnitTest(t)
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
	tf.UnitTest(t)
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
	tf.UnitTest(t)
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

	type case7 struct {
		A AUInt `json:"a"`
		B AInt
	}

	cases := []similarCase{
		{
			val:     new(case1),
			smode:   StructFieldsOrdered,
			reasons: []error{ReasonStructField, ReasonExportedFieldName},
		},
		{
			val:     new(case1),
			smode:   0,
			reasons: []error{ReasonStructField, ReasonExportedFieldNotFound},
		},
		{
			val:     new(case2),
			smode:   StructFieldsOrdered,
			reasons: []error{ReasonStructField, ReasonExportedFieldName},
		},
		{
			val:   new(case2),
			smode: 0,
		},
		{
			val:     new(case3),
			smode:   StructFieldsOrdered,
			reasons: []error{ReasonStructField, ReasonExportedFieldName},
		},
		{
			val:   new(case3),
			smode: 0,
		},
		{
			val:   new(case4),
			smode: 0,
		},
		{
			val:   new(case4),
			smode: StructFieldsOrdered,
		},
		{
			val:     new(case5),
			smode:   0,
			reasons: []error{ReasonStructField, ReasonExportedFieldsCount},
		},
		{
			val:     new(case5),
			smode:   StructFieldsOrdered,
			reasons: []error{ReasonStructField, ReasonExportedFieldsCount},
		},
		{
			val:     new(case6),
			smode:   0,
			reasons: []error{ReasonStructField, ReasonExportedFieldType},
		},
		{
			val:     new(case6),
			smode:   StructFieldsOrdered,
			reasons: []error{ReasonStructField, ReasonExportedFieldType},
		},
		{
			val:     new(case7),
			smode:   StructFieldTagsMatch,
			reasons: []error{ReasonStructField, ReasonExportedFieldTag},
		},
		{
			val:     new(case7),
			smode:   StructFieldsOrdered | StructFieldTagsMatch,
			reasons: []error{ReasonStructField, ReasonExportedFieldTag},
		},
	}

	similarTest(t, new(origin), cases, true)
}

func TestInterface(t *testing.T) {
	tf.UnitTest(t)
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
		{
			val:     new(case9),
			smode:   InterfaceAllMethods,
			reasons: []error{ReasonExportedMethodsCount},
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
	tf.UnitTest(t)
	cases := []similarCase{
		{
			val:       new(AInt),
			codecFlag: 0,
		},
		{
			val:       new(AInt),
			codecFlag: CodecBinary,
			reasons:   []error{ReasonCodecMarshalerImplementations},
		},
		{
			val:       new(AInt),
			codecFlag: CodecText,
			reasons:   []error{ReasonCodecMarshalerImplementations},
		},
		{
			val:       new(AInt),
			codecFlag: CodecJSON,
			reasons:   []error{ReasonCodecMarshalerImplementations},
		},
		{
			val:       new(AInt),
			codecFlag: CodecCbor,
			reasons:   []error{ReasonCodecMarshalerImplementations},
		},
		{
			val:       new(codecInt),
			codecFlag: 0,
		},
		{
			val:       new(codecInt),
			codecFlag: CodecBinary,
			reasons:   []error{ReasonCodecUnmarshalerImplementations},
		},
		{
			val:       new(codecInt),
			codecFlag: CodecText,
			reasons:   []error{ReasonCodecUnmarshalerImplementations},
		},
		{
			val:       new(codecInt),
			codecFlag: CodecJSON,
			reasons:   []error{ReasonCodecUnmarshalerImplementations},
		},
		{
			val:       new(codecInt),
			codecFlag: CodecCbor,
			reasons:   []error{ReasonCodecUnmarshalerImplementations},
		},
	}

	similarTest(t, new(halfCodecInt), cases, false)
}

func TestConvertible(t *testing.T) {
	tf.UnitTest(t)
	type origin struct {
		A uint
		B int
	}

	type another origin

	yes, reason := Similar(new(origin), new(another), 0, 0)
	require.Truef(t, yes, "convertible types, got reason: %s", reason)

	type ra = io.ReadCloser
	type rb = io.Reader
	rta := reflect.TypeOf(new(ra)).Elem()
	rtb := reflect.TypeOf(new(rb)).Elem()
	require.True(t, rta.ConvertibleTo(rtb))

	yes, reason = Similar(rta, rtb, 0, 0)
	require.False(t, yes, "convertible interface may not be similar")
	require.True(t, errors.Is(reason, ReasonExportedMethodsCount))
}

func TestRecursive(t *testing.T) {
	tf.UnitTest(t)
	type origin struct {
		A   uint
		B   int
		Sub []origin
	}

	type case1 struct {
		A   uint
		B   int
		Sub []case1
	}

	type case2 struct {
		A   uint
		B   int
		Sub []origin
	}

	cases := []similarCase{
		{
			val:   new(case1),
			smode: 0,
		},
		{
			val:     new(case1),
			smode:   AvoidRecursive,
			reasons: []error{ReasonRecursiveCompare},
		},
		{
			val:   new(case2),
			smode: 0,
		},
		{
			val:   new(case2),
			smode: AvoidRecursive,
		},
	}

	similarTest(t, new(origin), cases, false)
}
