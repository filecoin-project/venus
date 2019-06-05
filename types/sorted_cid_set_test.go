package types

import (
	"encoding/json"
	cid "github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"testing"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestSortedCidSet(t *testing.T) {
	tf.UnitTest(t)

	s := SortedCidSet{}

	assert.Equal(t, 0, s.Len())
	assert.True(t, s.Empty())

	// Iterate empty set is fine
	it := s.Iter()
	assert.Equal(t, it.Value(), cid.Undef)
	assert.False(t, it.Next())

	c1, _ := cid.Parse("zDPWYqFD4b5HLFuPfhkjJJkfvm4r8KLi1V9e2ahJX6Ab16Ay24pJ")
	c2, _ := cid.Parse("zDPWYqFD4b5HLFuPfhkjJJkfvm4r8KLi1V9e2ahJX6Ab16Ay24pK")
	c3, _ := cid.Parse("zDPWYqFD4b5HLFuPfhkjJJkfvm4r8KLi1V9e2ahJX6Ab16Ay24pL")

	// TODO: could test this more extensively -- version, codec, etc.
	assert.True(t, cidLess(c1, c2))
	assert.True(t, cidLess(c2, c3))
	assert.True(t, cidLess(c1, c3))
	assert.False(t, cidLess(c1, c1))
	assert.False(t, cidLess(c2, c1))

	assert.False(t, s.Has(c2))

	assert.True(t, s.Add(c2))
	assert.True(t, s.Has(c2))
	assert.Equal(t, 1, s.Len())
	assert.False(t, s.Empty())

	assert.False(t, s.Add(c2))

	assert.True(t, s.Add(c3))
	assert.True(t, s.Add(c1))

	assert.Equal(t, 3, s.Len())
	it = s.Iter()
	assert.True(t, c1.Equals(it.Value()))
	assert.True(t, it.Next())
	assert.True(t, c2.Equals(it.Value()))
	assert.True(t, it.Next())
	assert.True(t, c3.Equals(it.Value()))
	assert.False(t, it.Next())
	assert.Equal(t, it.Value(), cid.Undef)
	assert.True(t, it.Complete())

	assert.True(t, s.Remove(c2))
	assert.Equal(t, 2, s.Len())
	assert.False(t, s.Empty())

	assert.False(t, s.Remove(c2))
	assert.Equal(t, 2, s.Len())

	s.Clear()
	assert.Equal(t, 0, s.Len())
	assert.True(t, s.Empty())
}

func TestSortedCidSetCborRoundtrip(t *testing.T) {
	tf.UnitTest(t)

	exp := SortedCidSet{}
	makeCid := NewCidForTestGetter()
	exp.Add(makeCid())
	exp.Add(makeCid())
	exp.Add(makeCid())

	buf, err := cbor.DumpObject(exp)
	assert.NoError(t, err)

	var act SortedCidSet
	err = cbor.DecodeInto(buf, &act)
	assert.NoError(t, err)

	assert.Equal(t, 3, act.Len())
	assert.True(t, act.Equals(exp))
}

func TestSortedCidSetJSONRoundtrip(t *testing.T) {
	tf.UnitTest(t)

	exp := SortedCidSet{}
	makeCid := NewCidForTestGetter()
	exp.Add(makeCid())
	exp.Add(makeCid())
	exp.Add(makeCid())

	buf, err := json.Marshal(exp)
	assert.NoError(t, err)

	var act SortedCidSet
	err = json.Unmarshal(buf, &act)
	assert.NoError(t, err)

	assert.Equal(t, 3, act.Len())
	assert.True(t, act.Equals(exp))
}
