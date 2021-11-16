package testutil

import (
	"encoding/hex"
	"math/rand"
	"testing"
)

func init() {
	MustRegisterDefaultValueProvier(IntProvider)
	MustRegisterDefaultValueProvier(Int64Provider)
	MustRegisterDefaultValueProvier(Int32Provider)
	MustRegisterDefaultValueProvier(Int16Provider)
	MustRegisterDefaultValueProvier(Int8Provider)

	MustRegisterDefaultValueProvier(UintProvider)
	MustRegisterDefaultValueProvier(Uint64Provider)
	MustRegisterDefaultValueProvier(Uint32Provider)
	MustRegisterDefaultValueProvier(Uint16Provider)
	MustRegisterDefaultValueProvier(Uint8Provider)

	MustRegisterDefaultValueProvier(BytesFixedProvider(defaultBytesFixedSize))
	MustRegisterDefaultValueProvier(StringInnerFixedProvider(defaultBytesFixedSize))
}

const (
	defaultBytesFixedSize = 16
)

const (
	mask8  = 1<<8 - 1
	mask16 = 1<<16 - 1
	mask32 = 1<<32 - 1
)

func IntProvider(t *testing.T) int     { return rand.Int() }
func Int64Provider(t *testing.T) int64 { return int64(IntProvider(t)) }
func Int32Provider(t *testing.T) int32 { return int32(IntProvider(t) & mask32) }
func Int16Provider(t *testing.T) int16 { return int16(IntProvider(t) & mask16) }
func Int8Provider(t *testing.T) int8   { return int8(IntProvider(t) & mask8) }

func UintProvider(t *testing.T) uint     { return uint(IntProvider(t)) }
func Uint64Provider(t *testing.T) uint64 { return uint64(Int64Provider(t)) }
func Uint32Provider(t *testing.T) uint32 { return uint32(Int32Provider(t)) }
func Uint16Provider(t *testing.T) uint16 { return uint16(Int16Provider(t)) }
func Uint8Provider(t *testing.T) uint8   { return uint8(Int8Provider(t)) }

func BytesFixedProvider(size int) func(*testing.T) []byte {
	return func(t *testing.T) []byte {
		b := make([]byte, size)
		rand.Read(b[:])
		return b
	}
}

func BytesAtMostProvider(size int) func(*testing.T) []byte {
	return func(t *testing.T) []byte {
		b := make([]byte, rand.Intn(size))
		rand.Read(b[:])
		return b
	}
}

func StringInnerFixedProvider(size int) func(*testing.T) string {
	return func(t *testing.T) string {
		return hex.EncodeToString(BytesFixedProvider(size)(t))
	}
}

func StringInnerAtMostProvider(size int) func(*testing.T) string {
	return func(t *testing.T) string {
		return hex.EncodeToString(BytesFixedProvider(size)(t))
	}
}

func IntAtMostProvider(n int) func(*testing.T) int {
	return func(t *testing.T) int {
		if n <= 0 {
			t.Fatalf("get non-positive limit number %d", n)
		}

		return rand.Intn(n)
	}
}

func Int64AtMostProvider(n int64) func(*testing.T) int64 {
	return func(t *testing.T) int64 {
		return int64(IntAtMostProvider(int(n))(t))
	}
}

func Int32AtMostProvider(n int32) func(*testing.T) int32 {
	return func(t *testing.T) int32 {
		return int32(IntAtMostProvider(int(n))(t))
	}
}

func Int16AtMostProvider(n int16) func(*testing.T) int16 {
	return func(t *testing.T) int16 {
		return int16(IntAtMostProvider(int(n))(t))
	}
}

func Int8AtMostProvider(n int8) func(*testing.T) int8 {
	return func(t *testing.T) int8 {
		return int8(IntAtMostProvider(int(n))(t))
	}
}

func UintAtMostProvider(n uint) func(*testing.T) uint {
	return func(t *testing.T) uint {
		return uint(IntAtMostProvider(int(n))(t))
	}
}

func Uint64AtMostProvider(n uint64) func(*testing.T) uint64 {
	return func(t *testing.T) uint64 {
		return uint64(IntAtMostProvider(int(n))(t))
	}
}

func Uint32AtMostProvider(n uint32) func(*testing.T) uint32 {
	return func(t *testing.T) uint32 {
		return uint32(IntAtMostProvider(int(n))(t))
	}
}

func Uint16AtMostProvider(n uint16) func(*testing.T) uint16 {
	return func(t *testing.T) uint16 {
		return uint16(IntAtMostProvider(int(n))(t))
	}
}

func Uint8AtMostProvider(n uint8) func(*testing.T) uint8 {
	return func(t *testing.T) uint8 {
		return uint8(IntAtMostProvider(int(n))(t))
	}
}
