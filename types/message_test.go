package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessageMarshal(t *testing.T) {
	assert := assert.New(t)
	msg := Message{
		To:     Address("Alice"),
		From:   Address("Bob"),
		Method: "send",
		Params: []interface{}{"1", "2"},
	}

	marshalled, err := msg.Marshal()
	assert.NoError(err)

	msgBack := Message{}
	err = msgBack.Unmarshal(marshalled)
	assert.NoError(err)

	assert.Equal(msg.To, msgBack.To)
	assert.Equal(msg.From, msgBack.From)
	assert.Equal(msg.Method, msgBack.Method)
	assert.Equal(msg.Params, msgBack.Params)
}

func TestMessageCid(t *testing.T) {
	assert := assert.New(t)

	msg1 := Message{
		To:     Address("Alice1"),
		From:   Address("Bob"),
		Method: "send",
		Params: []interface{}{"1", "2"},
	}

	msg2 := Message{
		To:     Address("Alice2"),
		From:   Address("Bob"),
		Method: "send",
		Params: []interface{}{"1", "2"},
	}

	c1, err := msg1.Cid()
	assert.NoError(err)
	c2, err := msg2.Cid()
	assert.NoError(err)

	assert.NotEqual(c1.String(), c2.String())
}
