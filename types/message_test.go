package types

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessageMarshal(t *testing.T) {
	assert := assert.New(t)
	msg := NewMessage(
		Address("Alice"),
		Address("Bob"),
		big.NewInt(17777),
		"send",
		[]interface{}{"1", "2"},
	)

	marshalled, err := msg.Marshal()
	assert.NoError(err)

	msgBack := Message{}
	err = msgBack.Unmarshal(marshalled)
	assert.NoError(err)

	assert.Equal(msg.To, msgBack.To)
	assert.Equal(msg.From, msgBack.From)
	assert.Equal(msg.Value, msgBack.Value)
	assert.Equal(msg.Method, msgBack.Method)
	assert.Equal(msg.Params, msgBack.Params)
}

func TestMessageCid(t *testing.T) {
	assert := assert.New(t)

	msg1 := NewMessage(
		Address("Alice1"),
		Address("Bob"),
		big.NewInt(999),
		"send",
		[]interface{}{"1", "2"},
	)

	msg2 := NewMessage(
		Address("Alice2"),
		Address("Bob"),
		big.NewInt(4004),
		"send",
		[]interface{}{"1", "2"},
	)

	c1, err := msg1.Cid()
	assert.NoError(err)
	c2, err := msg2.Cid()
	assert.NoError(err)

	assert.NotEqual(c1.String(), c2.String())
}

func TestNewMessageForTestGetter(t *testing.T) {
	newMsg := NewMessageForTestGetter()
	m1 := newMsg()
	c1, _ := m1.Cid()
	m2 := newMsg()
	c2, _ := m2.Cid()
	assert.False(t, c1.Equals(c2))
}
