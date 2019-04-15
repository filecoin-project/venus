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

	assert := assert.New(t)

	s := SortedCidSet{}

	assert.Equal(0, s.Len())
	assert.True(s.Empty())

	// Iterate empty set is fine
	it := s.Iter()
	assert.Equal(it.Value(), cid.Undef)
	assert.False(it.Next())

	c1, _ := cid.Parse("zDPWYqFD4b5HLFuPfhkjJJkfvm4r8KLi1V9e2ahJX6Ab16Ay24pJ")
	c2, _ := cid.Parse("zDPWYqFD4b5HLFuPfhkjJJkfvm4r8KLi1V9e2ahJX6Ab16Ay24pK")
	c3, _ := cid.Parse("zDPWYqFD4b5HLFuPfhkjJJkfvm4r8KLi1V9e2ahJX6Ab16Ay24pL")

	// TODO: could test this more extensively -- version, codec, etc.
	assert.True(cidLess(c1, c2))
	assert.True(cidLess(c2, c3))
	assert.True(cidLess(c1, c3))
	assert.False(cidLess(c1, c1))
	assert.False(cidLess(c2, c1))

	assert.False(s.Has(c2))

	assert.True(s.Add(c2))
	assert.True(s.Has(c2))
	assert.Equal(1, s.Len())
	assert.False(s.Empty())

	assert.False(s.Add(c2))

	assert.True(s.Add(c3))
	assert.True(s.Add(c1))

	assert.Equal(3, s.Len())
	it = s.Iter()
	assert.True(c1.Equals(it.Value()))
	assert.True(it.Next())
	assert.True(c2.Equals(it.Value()))
	assert.True(it.Next())
	assert.True(c3.Equals(it.Value()))
	assert.False(it.Next())
	assert.Equal(it.Value(), cid.Undef)
	assert.True(it.Complete())

	assert.True(s.Remove(c2))
	assert.Equal(2, s.Len())
	assert.False(s.Empty())

	assert.False(s.Remove(c2))
	assert.Equal(2, s.Len())

	s.Clear()
	assert.Equal(0, s.Len())
	assert.True(s.Empty())
}

func TestSortedCidSetCborRoundtrip(t *testing.T) {
	tf.UnitTest(t)

	assert := assert.New(t)

	exp := SortedCidSet{}
	makeCid := NewCidForTestGetter()
	exp.Add(makeCid())
	exp.Add(makeCid())
	exp.Add(makeCid())

	buf, err := cbor.DumpObject(exp)
	assert.NoError(err)

	var act SortedCidSet
	err = cbor.DecodeInto(buf, &act)
	assert.NoError(err)

	assert.Equal(3, act.Len())
	assert.True(act.Equals(exp))
}

func TestSortedCidSetJSONRoundtrip(t *testing.T) {
	tf.UnitTest(t)

	assert := assert.New(t)

	exp := SortedCidSet{}
	makeCid := NewCidForTestGetter()
	exp.Add(makeCid())
	exp.Add(makeCid())
	exp.Add(makeCid())

	buf, err := json.Marshal(exp)
	assert.NoError(err)

	var act SortedCidSet
	err = json.Unmarshal(buf, &act)
	assert.NoError(err)

	assert.Equal(3, act.Len())
	assert.True(act.Equals(exp))
}
