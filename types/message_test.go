package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessageMarshal(t *testing.T) {
	assert := assert.New(t)
	msg := Message{
		to:     Address("Alice"),
		from:   Address("Bob"),
		method: "send",
		params: []interface{}{"1", "2"},
	}

	marshalled, err := msg.Marshal()
	assert.NoError(err)

	msgBack := Message{}
	err = msgBack.Unmarshal(marshalled)
	assert.NoError(err)

	assert.Equal(msg.To(), msgBack.To())
	assert.Equal(msg.From(), msgBack.From())
	assert.Equal(msg.Method(), msgBack.Method())
	assert.Equal(msg.Params(), msgBack.Params())
}

func TestMessageCid(t *testing.T) {
	assert := assert.New(t)

	msg1 := Message{
		to:     Address("Alice1"),
		from:   Address("Bob"),
		method: "send",
		params: []interface{}{"1", "2"},
	}

	msg2 := Message{
		to:     Address("Alice2"),
		from:   Address("Bob"),
		method: "send",
		params: []interface{}{"1", "2"},
	}

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

func TestMessageHasFrom(t *testing.T) {
	assert := assert.New(t)

	msgNoFrom := NewMessage(Address(""), Address("to"), "balance", nil)
	msgFrom := NewMessage(Address("from"), Address("to"), "balance", nil)

	assert.False(msgNoFrom.HasFrom())
	assert.True(msgFrom.HasFrom())
}
