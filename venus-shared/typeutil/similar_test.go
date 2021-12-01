package typeutil

import (
	"context"
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
	why       string
	codecFlag CodecFlag
	ordered   Ordered
	expected  bool
}

func similarTest(t *testing.T, origin interface{}, cases []similarCase, checkIndirect bool) {
	valOrigin := reflect.ValueOf(origin)
	typOrigin := valOrigin.Type()
	indirect := checkIndirect && reflect.Indirect(valOrigin).Type() != typOrigin

	for i := range cases {
		typCase := reflect.TypeOf(cases[i].val)
		require.NotEqual(t, typOrigin, typCase, "types should be different")
		require.Equal(t, typOrigin.Kind(), typCase.Kind(), "kind should not be different")

		yes, reason := Similar(typOrigin, typCase, cases[i].codecFlag, cases[i].ordered)
		require.Equalf(t, cases[i].expected, yes, "expected result of %s <-> %s: %v as a result of %s, got reason: %s", typOrigin, typCase, cases[i].expected, cases[i].why, reason)

		if indirect {
			require.Equal(t, typOrigin.Elem().Kind(), typCase.Elem().Kind(), "kind of indirect type should not be different")

			yes, reason = Similar(typOrigin.Elem(), typCase.Elem(), cases[i].codecFlag, cases[i].ordered)
			require.Equalf(t, cases[i].expected, yes, "expected result for indirect types %s <-> %s: %v as a result of %s, got reason: %s", typOrigin.Elem(), typCase.Elem(), cases[i].expected, cases[i].why, reason)
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
			val:      new(case1),
			why:      "array element",
			expected: false,
		},
		{
			val:      new(case2),
			why:      "array len",
			expected: false,
		},
		{
			val:      new(case3),
			expected: true,
		},
		{
			val:      new(case4),
			why:      "array len",
			expected: false,
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
			val:      new(case1),
			why:      "map key",
			expected: false,
		},
		{
			val:      new(case2),
			why:      "map value",
			expected: false,
		},
		{
			val:      new(case3),
			expected: true,
		},
		{
			val:      new(case4),
			expected: true,
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
			val:      new(case1),
			why:      "slice element",
			expected: false,
		},
		{
			val:      new(case2),
			why:      "slice element",
			expected: false,
		},
		{
			val:      new(case3),
			expected: true,
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
			val:      new(case1),
			why:      "elem type",
			expected: false,
		},
		{
			val:      new(case2),
			why:      "chan dir",
			expected: false,
		},
		{
			val:      new(case3),
			why:      "chan dir",
			expected: false,
		},
		{
			val:      new(case4),
			expected: true,
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
			val:      new(case1),
			ordered:  StructFieldsOrdered,
			expected: true,
		},
		{
			val:      new(case1),
			ordered:  0,
			why:      "name diverse",
			expected: false,
		},
		{
			val:      new(case2),
			ordered:  0,
			expected: true,
		},
		{
			val:      new(case2),
			ordered:  StructFieldsOrdered,
			why:      "order not match",
			expected: false,
		},
		{
			val:      new(case3),
			ordered:  0,
			expected: true,
		},
		{
			val:      new(case3),
			ordered:  StructFieldsOrdered,
			why:      "order not match",
			expected: false,
		},
		{
			val:      new(case4),
			ordered:  0,
			expected: true,
		},
		{
			val:      new(case4),
			ordered:  StructFieldsOrdered,
			expected: true,
		},
		{
			val:      new(case5),
			ordered:  0,
			why:      "field count",
			expected: false,
		},
		{
			val:      new(case5),
			ordered:  StructFieldsOrdered,
			why:      "field count",
			expected: false,
		},
		{
			val:      new(case6),
			ordered:  0,
			why:      "field type",
			expected: false,
		},
		{
			val:      new(case6),
			ordered:  StructFieldsOrdered,
			why:      "field type",
			expected: false,
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

	cases := []similarCase{
		{
			val:      new(case1),
			expected: true,
		},
		{
			val:      new(case2),
			why:      "method name",
			expected: false,
		},
		{
			val:      new(case3),
			why:      "in num",
			expected: false,
		},
		{
			val:      new(case4),
			why:      "out num",
			expected: false,
		},
		{
			val:      new(case5),
			expected: true,
		},
		{
			val:      new(case6),
			why:      "method num",
			expected: false,
		},
		{
			val:      new(case7),
			why:      "out type",
			expected: false,
		},
		{
			val:      new(case8),
			why:      "in type",
			expected: false,
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
			val:       new(codecInt),
			codecFlag: 0,
			expected:  true,
		},
		{
			val:       new(codecInt),
			codecFlag: BinaryCodec,
			why:       "binary codec",
			expected:  false,
		},
		{
			val:       new(codecInt),
			codecFlag: TextCodec,
			why:       "text codec",
			expected:  false,
		},
		{
			val:       new(codecInt),
			codecFlag: JSONCodec,
			why:       "json codec",
			expected:  false,
		},
		{
			val:       new(codecInt),
			codecFlag: CborCodec,
			why:       "cbor codec",
			expected:  false,
		},
	}

	similarTest(t, new(AInt), cases, false)
	similarTest(t, new(halfCodecInt), cases, false)
}

func TestUnexpectedKind(t *testing.T) {
	type Uintptr uintptr
	yes, _ := Similar(new(uintptr), new(Uintptr), 0, 0)
	require.False(t, yes, "uintptr is unexpected")

	type UnsafePointer unsafe.Pointer
	yes, _ = Similar(new(unsafe.Pointer), new(UnsafePointer), 0, 0)
	require.False(t, yes, "unsafe.Pointer is unexpected")
}
