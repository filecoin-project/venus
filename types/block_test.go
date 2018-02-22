package types

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlockAddParent(t *testing.T) {
	var p, c Block
	assert.False(t, p.IsParentOf(c))
	assert.False(t, c.IsParentOf(p))

	// c.Height is 0
	err := c.AddParent(p)
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "height")
	}
	c.Height = 100
	err = c.AddParent(p)
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "height")
	}

	c.Height = p.Height + 1
	assert.NoError(t, c.AddParent(p))
	assert.True(t, p.IsParentOf(c))
	assert.False(t, c.IsParentOf(p))
}

func TestDecodeBlock(t *testing.T) {
	assert := assert.New(t)

	m := NewMessage(Address("from"), Address("to"), big.NewInt(10), "hello", []interface{}{"world", big.NewInt(11)})
	mCid, err := m.Cid()
	assert.NoError(err)

	c1, err := cidFromString("a")
	assert.NoError(err)
	c2, err := cidFromString("b")
	assert.NoError(err)

	before := Block{
		Parent:          c1,
		Height:          2,
		Messages:        []*Message{m},
		StateRoot:       c2,
		MessageReceipts: []*MessageReceipt{NewMessageReceipt(mCid, 1, []byte{1, 2})},
	}

	after, err := DecodeBlock(before.ToNode().RawData())
	assert.NoError(err)
	assert.Equal(after.Cid(), before.Cid())
	assert.Equal(after, before)
}
