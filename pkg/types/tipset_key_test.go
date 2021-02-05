package types

import (
	"bytes"
	"encoding/json"
	"testing"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

func TestTipSetKey(t *testing.T) {
	tf.UnitTest(t)

	c1, _ := cid.Parse("zDPWYqFD4b5HLFuPfhkjJJkfvm4r8KLi1V9e2ahJX6Ab16Ay24pJ")
	c2, _ := cid.Parse("zDPWYqFD4b5HLFuPfhkjJJkfvm4r8KLi1V9e2ahJX6Ab16Ay24pK")
	c3, _ := cid.Parse("zDPWYqFD4b5HLFuPfhkjJJkfvm4r8KLi1V9e2ahJX6Ab16Ay24pL")
	c4, _ := cid.Parse("zDPWYqFD4b5HLFuPfhkjJJkfvm4r8KLi1V9e2ahJX6Ab16Ay24pM")

	t.Run("contains", func(t *testing.T) {
		empty := NewTipSetKey()
		s := NewTipSetKey(c1, c2, c3)

		assert.False(t, empty.Has(c1))
		assert.True(t, s.Has(c1))
		assert.True(t, s.Has(c2))
		assert.True(t, s.Has(c3))
		assert.False(t, s.Has(c4))

		assert.True(t, s.ContainsAll(empty))
		assert.True(t, s.ContainsAll(NewTipSetKey(c1)))
		assert.True(t, s.ContainsAll(s))
		assert.False(t, s.ContainsAll(NewTipSetKey(c4)))
		assert.False(t, s.ContainsAll(NewTipSetKey(c1, c4)))

		assert.True(t, empty.ContainsAll(empty))
		assert.False(t, empty.ContainsAll(s))
	})
}

func TestTipSetKeyCborRoundtrip(t *testing.T) {
	tf.UnitTest(t)

	makeCid := NewCidForTestGetter()
	exp := NewTipSetKey(makeCid(), makeCid(), makeCid())
	buf := new(bytes.Buffer)
	err := exp.MarshalCBOR(buf)
	assert.NoError(t, err)

	var act TipSetKey
	err = act.UnmarshalCBOR(buf)
	assert.NoError(t, err)

	assert.Equal(t, 3, len(act.Cids()))
	assert.True(t, act.Equals(exp))
}

func TestTipSetKeyJSONRoundtrip(t *testing.T) {
	tf.UnitTest(t)

	makeCid := NewCidForTestGetter()
	exp := NewTipSetKey(makeCid(), makeCid(), makeCid())

	buf, err := json.Marshal(exp)
	assert.NoError(t, err)

	var act TipSetKey
	err = json.Unmarshal(buf, &act)
	assert.NoError(t, err)

	assert.Equal(t, 3, len(act.Cids()))
	assert.True(t, act.Equals(exp))
}
