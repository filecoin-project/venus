package types

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBlockIsParentOf(t *testing.T) {
	var p, c Block
	assert.False(t, p.IsParentOf(c))
	assert.False(t, c.IsParentOf(p))

	c.Parents.Add(p.Cid())
	assert.True(t, p.IsParentOf(c))
	assert.False(t, c.IsParentOf(p))
}

func TestBlockScore(t *testing.T) {
	source := rand.NewSource(time.Now().UnixNano())

	t.Run("block score equals block height", func(t *testing.T) {
		assert := assert.New(t)

		for i := 0; i < 100; i++ {
			n := uint64(source.Int63())

			var b Block
			b.Height = n

			assert.Equal(b.Height, b.Score(), "block height: %d - block score %d", b.Height, b.Score())
		}
	})
}

func TestDecodeBlock(t *testing.T) {
	t.Run("successfully decodes raw bytes to a Filecoin block", func(t *testing.T) {
		assert := assert.New(t)

		addrGetter := NewAddressForTestGetter()
		m1 := NewMessage(addrGetter(), addrGetter(), 0, NewAttoFILFromFIL(10), "hello", []byte("cat"))
		m2 := NewMessage(addrGetter(), addrGetter(), 0, NewAttoFILFromFIL(2), "yes", []byte("dog"))

		c1, err := cidFromString("a")
		assert.NoError(err)
		c2, err := cidFromString("b")
		assert.NoError(err)

		before := &Block{
			Parents:   NewSortedCidSet(c1),
			Height:    2,
			Messages:  []*Message{m1, m2},
			StateRoot: c2,
			MessageReceipts: []*MessageReceipt{
				{ExitCode: 1, Return: []Bytes{[]byte{1, 2}}},
				{ExitCode: 1, Return: []Bytes{[]byte{1, 2, 3}}},
			},
		}

		after, err := DecodeBlock(before.ToNode().RawData())
		assert.NoError(err)
		assert.Equal(after.Cid(), before.Cid())
		assert.Equal(after, before)
	})

	t.Run("decode failure results in an error", func(t *testing.T) {
		assert := assert.New(t)

		_, err := DecodeBlock([]byte{1, 2, 3})
		assert.Error(err)
		assert.Contains(err.Error(), "malformed stream")
	})
}

func TestEquals(t *testing.T) {
	assert := assert.New(t)

	c1, err := cidFromString("a")
	assert.NoError(err)
	c2, err := cidFromString("b")
	assert.NoError(err)

	var n1 uint64 = 1234
	var n2 uint64 = 9876

	b1 := &Block{Parents: NewSortedCidSet(c1), Nonce: n1}
	b2 := &Block{Parents: NewSortedCidSet(c1), Nonce: n1}
	b3 := &Block{Parents: NewSortedCidSet(c1), Nonce: n2}
	b4 := &Block{Parents: NewSortedCidSet(c2), Nonce: n1}
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
	child.Parents = NewSortedCidSet(parent.Cid())
	child.StateRoot = parent.Cid()

	mkMsg := NewMessageForTestGetter()

	message := mkMsg()

	receipt := &MessageReceipt{ExitCode: 0}
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
