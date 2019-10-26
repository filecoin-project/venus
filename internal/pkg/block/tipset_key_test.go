package block_test

import (
	"encoding/json"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blk "github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

func TestTipSetKey(t *testing.T) {
	tf.UnitTest(t)

	c1, _ := cid.Parse("zDPWYqFD4b5HLFuPfhkjJJkfvm4r8KLi1V9e2ahJX6Ab16Ay24pJ")
	c2, _ := cid.Parse("zDPWYqFD4b5HLFuPfhkjJJkfvm4r8KLi1V9e2ahJX6Ab16Ay24pK")
	c3, _ := cid.Parse("zDPWYqFD4b5HLFuPfhkjJJkfvm4r8KLi1V9e2ahJX6Ab16Ay24pL")
	c4, _ := cid.Parse("zDPWYqFD4b5HLFuPfhkjJJkfvm4r8KLi1V9e2ahJX6Ab16Ay24pM")

	t.Run("empty", func(t *testing.T) {
		s := blk.NewTipSetKey()
		assert.True(t, s.Empty())
		assert.Equal(t, 0, s.Len())

		it := s.Iter()
		assert.Equal(t, it.Value(), cid.Undef)
		assert.False(t, it.Next())
	})

	t.Run("zero value is empty", func(t *testing.T) {
		var s blk.TipSetKey
		assert.True(t, s.Empty())
		assert.Equal(t, 0, s.Len())

		it := s.Iter()
		assert.Equal(t, it.Value(), cid.Undef)
		assert.False(t, it.Next())

		assert.True(t, s.Equals(blk.NewTipSetKey()))

		// Bytes must be equal in order to have equivalent CIDs
		zeroBytes, err := encoding.Encode(s)
		require.NoError(t, err)
		emptyBytes, err := encoding.Encode(blk.NewTipSetKey())
		require.NoError(t, err)
		assert.Equal(t, zeroBytes, emptyBytes)
	})

	t.Run("order invariant", func(t *testing.T) {
		s1 := blk.NewTipSetKey(c1, c2, c3)
		s2 := blk.NewTipSetKey(c3, c2, c1)

		assert.True(t, s1.Equals(s2))

		// Sorted order is not a defined property, but an important implementation detail to
		// verify unless the implementation is changed.
		assert.Equal(t, []cid.Cid{c1, c2, c3}, s1.ToSlice())
		assert.Equal(t, []cid.Cid{c1, c2, c3}, s2.ToSlice())
	})

	t.Run("drops duplicates", func(t *testing.T) {
		cases := [][]cid.Cid{
			{c1},
			{c1, c1, c1},
			{c1, c2, c3},
			{c1, c1, c2, c3},
			{c1, c2, c2, c3},
			{c1, c2, c3, c3},
			{c1, c1, c2, c2, c3, c3},
		}

		for _, cs := range cases {
			cidSet := asSet(cs)
			key := blk.NewTipSetKey(cs...)
			assert.Equal(t, len(cidSet), key.Len())
			assert.Equal(t, cidSet, asSet(key.ToSlice()))
			assert.Equal(t, key, blk.NewTipSetKey(asSlice(cidSet)...))
		}
	})

	t.Run("fails if unexpected duplicates", func(t *testing.T) {
		_, e := blk.NewTipSetKeyFromUnique(c1, c2, c3)
		assert.NoError(t, e)
		_, e = blk.NewTipSetKeyFromUnique(c1, c2, c1, c3)
		assert.Error(t, e)
	})

	t.Run("contains", func(t *testing.T) {
		empty := blk.NewTipSetKey()
		s := blk.NewTipSetKey(c1, c2, c3)

		assert.False(t, empty.Has(c1))
		assert.True(t, s.Has(c1))
		assert.True(t, s.Has(c2))
		assert.True(t, s.Has(c3))
		assert.False(t, s.Has(c4))

		assert.True(t, s.ContainsAll(empty))
		assert.True(t, s.ContainsAll(blk.NewTipSetKey(c1)))
		assert.True(t, s.ContainsAll(s))
		assert.False(t, s.ContainsAll(blk.NewTipSetKey(c4)))
		assert.False(t, s.ContainsAll(blk.NewTipSetKey(c1, c4)))

		assert.True(t, empty.ContainsAll(empty))
		assert.False(t, empty.ContainsAll(s))
	})

	t.Run("iteration", func(t *testing.T) {
		s := blk.NewTipSetKey(c3, c2, c1)
		it := s.Iter()
		assert.True(t, c1.Equals(it.Value()))
		assert.True(t, it.Next())
		assert.True(t, c2.Equals(it.Value()))
		assert.True(t, it.Next())
		assert.True(t, c3.Equals(it.Value()))
		assert.False(t, it.Next())
		assert.Equal(t, it.Value(), cid.Undef)
		assert.True(t, it.Complete())
	})
}

func TestTipSetKeyCborRoundtrip(t *testing.T) {
	tf.UnitTest(t)

	makeCid := types.NewCidForTestGetter()
	exp := blk.NewTipSetKey(makeCid(), makeCid(), makeCid())
	buf, err := encoding.Encode(exp)
	assert.NoError(t, err)

	var act blk.TipSetKey
	err = encoding.Decode(buf, &act)
	assert.NoError(t, err)

	assert.Equal(t, 3, act.Len())
	assert.True(t, act.Equals(exp))
}

func TestTipSetKeyJSONRoundtrip(t *testing.T) {
	tf.UnitTest(t)

	makeCid := types.NewCidForTestGetter()
	exp := blk.NewTipSetKey(makeCid(), makeCid(), makeCid())

	buf, err := json.Marshal(exp)
	assert.NoError(t, err)

	var act blk.TipSetKey
	err = json.Unmarshal(buf, &act)
	assert.NoError(t, err)

	assert.Equal(t, 3, act.Len())
	assert.True(t, act.Equals(exp))
}

func asSet(cids []cid.Cid) map[cid.Cid]struct{} {
	set := make(map[cid.Cid]struct{})
	for _, c := range cids {
		set[c] = struct{}{}
	}
	return set
}

func asSlice(cids map[cid.Cid]struct{}) []cid.Cid {
	slc := make([]cid.Cid, len(cids))
	var i int
	for c := range cids {
		slc[i] = c
		i++
	}
	return slc
}
