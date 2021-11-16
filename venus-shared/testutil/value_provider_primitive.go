package testutil

import (
	"encoding/hex"
	"math/rand"
	"testing"
)

func init() {
	MustRegisterDefaultValueProvier(IntProvider)

	MustRegisterDefaultValueProvier(BytesFixedProvider(defaultBytesFixedSize))
	MustRegisterDefaultValueProvier(StringInnerFixedProvider(defaultBytesFixedSize))
}

const (
	defaultBytesFixedSize = 16
)

func IntProvider(t *testing.T) int { return rand.Int() }

func IntAtMostProvider(n int) func(*testing.T) int {
	return func(t *testing.T) int {
		if n <= 0 {
			t.Fatalf("get non-positive limit number %d", n)
		}

		return rand.Intn(n)
	}
}

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
