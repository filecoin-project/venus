package testutil

import (
	"encoding/hex"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func init() {
	MustRegisterDefaultValueProvier(IntProvider)

	MustRegisterDefaultValueProvier(BytesFixedProvider(defaultBytesFixedSize))
	MustRegisterDefaultValueProvier(StringInnerFixedProvider(defaultBytesFixedSize))
}

const (
	defaultBytesFixedSize = 16
)

func IntProvider(t *testing.T) int { return r.Int() }

func IntRangedProvider(min, max int) func(*testing.T) int {
	return func(t *testing.T) int {
		gap := max - min
		if gap <= 0 {
			t.Fatalf("invalid range [%d, %d)", min, max)
		}

		return min + r.Intn(gap)
	}
}

var r = rand.New(rand.NewSource(time.Now().UnixNano()))

func getRand() *rand.Rand {
	seed := time.Now().UnixNano()
	r = rand.New(rand.NewSource(seed))
	return rand.New(rand.NewSource(seed))
}

func BytesFixedProvider(size int) func(*testing.T) []byte {
	return func(t *testing.T) []byte {
		b := make([]byte, size)
		_, err := r.Read(b[:])
		require.NoError(t, err)
		return b
	}
}

func BytesAtMostProvider(size int) func(*testing.T) []byte {
	return func(t *testing.T) []byte {
		b := make([]byte, rand.Intn(size))
		_, err := r.Read(b[:])
		require.NoError(t, err)
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
