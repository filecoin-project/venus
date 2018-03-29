package types

import (
	"encoding/json"
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

	addrGetter := NewAddressForTestGetter()
	m1 := NewMessage(addrGetter(), addrGetter(), NewTokenAmount(10), "hello", []byte("cat"))
	m2 := NewMessage(addrGetter(), addrGetter(), NewTokenAmount(2), "yes", []byte("dog"))

	m1Cid, err := m1.Cid()
	assert.NoError(err)
	m2Cid, err := m2.Cid()
	assert.NoError(err)

	c1, err := cidFromString("a")
	assert.NoError(err)
	c2, err := cidFromString("b")
	assert.NoError(err)

	before := &Block{
		Parent:    c1,
		Height:    2,
		Messages:  []*Message{m1, m2},
		StateRoot: c2,
		MessageReceipts: []*MessageReceipt{
			NewMessageReceipt(m1Cid, 1, "", []byte{1, 2}),
			NewMessageReceipt(m2Cid, 1, "", []byte{1, 2}),
		},
	}

	after, err := DecodeBlock(before.ToNode().RawData())
	assert.NoError(err)
	assert.Equal(after.Cid(), before.Cid())
	assert.Equal(after, before)
}

func TestEquals(t *testing.T) {
	assert := assert.New(t)

	c1, err := cidFromString("a")
	assert.NoError(err)
	c2, err := cidFromString("b")
	assert.NoError(err)

	var n1 uint64 = 1234
	var n2 uint64 = 9876

	b1 := &Block{Parent: c1, Nonce: n1}
	b2 := &Block{Parent: c1, Nonce: n1}
	b3 := &Block{Parent: c1, Nonce: n2}
	b4 := &Block{Parent: c2, Nonce: n1}
	assert.True(b1.Equals(b1))
	assert.True(b1.Equals(b2))
	assert.False(b1.Equals(b3))
	assert.False(b1.Equals(b4))
	assert.False(b3.Equals(b4))
}

func TestBlockJsonMarshal(t *testing.T) {
	assert := assert.New(t)

	var parent, child Block
	child.Height = 1
	child.Nonce = 2
	child.Parent = parent.Cid()
	child.StateRoot = parent.Cid()

	mkMsg := NewMessageForTestGetter()

	message := mkMsg()
	receipt := NewMessageReceipt(SomeCid(), 0, "", nil)
	child.Messages = []*Message{message}
	child.MessageReceipts = []*MessageReceipt{receipt}

	marshalled, e1 := json.Marshal(child)
	assert.NoError(e1)
	str := string(marshalled)

	assert.Contains(str, parent.Cid().String())
	assert.Contains(str, message.From.String())
	assert.Contains(str, message.To.String())

	// marshal/unmarshal symmetry
	var unmarshalled Block
	e2 := json.Unmarshal(marshalled, &unmarshalled)
	assert.NoError(e2)

	assert.True(child.Equals(&unmarshalled))
}
