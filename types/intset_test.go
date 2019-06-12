package types_test

import (
	"testing"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

func TestIntSet(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Add and Contains", func(t *testing.T) {
		is := types.NewIntSet()
		is.Add(1)
		is.Add(0xffffffffffff)

		assert.True(t, is.Contains(1))
		assert.True(t, is.Contains(0xffffffffffff))
		assert.False(t, is.Contains(2))
	})

	t.Run("IsSubset", func(t *testing.T) {
		is0 := types.NewIntSet(1, 2, 3)
		is1 := types.NewIntSet()

		// {} ⊆ {1, 2, 3}
		assert.True(t, is0.IsSubset(is1))
		assert.False(t, is1.IsSubset(is0))

		// {3} ⊆ {1, 2, 3}
		is1.Add(3)
		assert.True(t, is0.IsSubset(is1))
		assert.False(t, is1.IsSubset(is0))

		// {3, 4} ⊈ {1, 2, 3}
		is1.Add(4)
		assert.False(t, is0.IsSubset(is1))
		assert.False(t, is1.IsSubset(is0))
	})

	t.Run("Union", func(t *testing.T) {
		is0 := types.NewIntSet(1)
		is1 := types.NewIntSet(2)

		result := is0.Union(is1)

		assert.True(t, result.Contains(1))
		assert.True(t, result.Contains(2))
	})

	t.Run("Intersection", func(t *testing.T) {
		is0 := types.NewIntSet(1, 2)
		is1 := types.NewIntSet(2, 3)

		result := is0.Intersection(is1)

		assert.True(t, result.Contains(2))
		assert.False(t, result.Contains(1))
		assert.False(t, result.Contains(3))
	})

	t.Run("Difference", func(t *testing.T) {
		is0 := types.NewIntSet(1, 2, 3)
		is1 := types.NewIntSet(3, 4, 5)

		result := is0.Difference(is1)

		assert.True(t, result.Contains(1))
		assert.True(t, result.Contains(2))
		assert.False(t, result.Contains(3))
	})

	t.Run("Integers", func(t *testing.T) {
		is0 := types.NewIntSet(1, 2, 3)

		assert.Equal(t, is0.Integers(), []uint64{1, 2, 3})
	})
}
