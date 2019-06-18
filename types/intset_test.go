package types_test

import (
	"math/rand"
	"sort"
	"testing"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

func TestIntSet(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Add", func(t *testing.T) {
		empty := types.NewIntSet()
		is := empty.Add(1).Add(0xffffffffffff)

		// doesn't mutate receivers
		assert.Equal(t, 0, len(empty.Values()))

		assert.True(t, is.Has(1))
		assert.True(t, is.Has(0xffffffffffff))
		assert.False(t, is.Has(2))
	})

	t.Run("HasSubset()", func(t *testing.T) {
		is0 := types.NewIntSet(1, 2, 3)
		is1 := types.NewIntSet()

		// {} ⊆ {1, 2, 3}
		assert.True(t, is0.HasSubset(is1))
		assert.False(t, is1.HasSubset(is0))

		// {3} ⊆ {1, 2, 3}
		is1 = is1.Add(3)
		assert.True(t, is0.HasSubset(is1))
		assert.False(t, is1.HasSubset(is0))

		// {3, 4} ⊈ {1, 2, 3}
		is1 = is1.Add(4)
		assert.False(t, is0.HasSubset(is1))
		assert.False(t, is1.HasSubset(is0))
	})

	t.Run("Union", func(t *testing.T) {
		is0 := types.NewIntSet(1)
		is1 := types.NewIntSet(2)

		result := is0.Union(is1)

		assert.True(t, result.Has(1))
		assert.True(t, result.Has(2))
	})

	t.Run("Intersection", func(t *testing.T) {
		is0 := types.NewIntSet(1, 2)
		is1 := types.NewIntSet(2, 3)

		result := is0.Intersection(is1)

		assert.True(t, result.Has(2))
		assert.False(t, result.Has(1))
		assert.False(t, result.Has(3))
	})

	t.Run("Difference", func(t *testing.T) {
		is0 := types.NewIntSet(1, 2, 3)
		is1 := types.NewIntSet(3, 4, 5)

		result := is0.Difference(is1)

		assert.True(t, result.Has(1))
		assert.True(t, result.Has(2))
		assert.False(t, result.Has(3))
	})

	t.Run("Values", func(t *testing.T) {
		ints := make([]uint64, 1024)
		for idx := range ints {
			ints[idx] = rand.Uint64()
		}

		result := types.NewIntSet(ints...).Values()

		sort.Slice(ints, func(i, j int) bool { return ints[i] < ints[j] })
		sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })

		assert.Equal(t, ints, result)
	})
}
