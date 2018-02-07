package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActorMarshal(t *testing.T) {
	assert := assert.New(t)
	actor := NewActorWithMemory(AccountActorCid, []byte{1, 2, 3})

	marshalled, err := actor.Marshal()
	assert.NoError(err)

	actorBack := Actor{}
	err = actorBack.Unmarshal(marshalled)
	assert.NoError(err)

	assert.Equal(actor.Code(), actorBack.Code())
	assert.Equal(actor.ReadStorage(), actorBack.ReadStorage())
	assert.Equal(actor.Nonce(), actorBack.Nonce())

	c1, err := actor.Cid()
	assert.NoError(err)
	c2, err := actorBack.Cid()
	assert.NoError(err)
	assert.Equal(c1, c2)
}

func TestActorCid(t *testing.T) {
	assert := assert.New(t)

	actor1 := NewActor(AccountActorCid)
	actor2 := NewActorWithMemory(AccountActorCid, []byte{1, 2, 3})

	c1, err := actor1.Cid()
	assert.NoError(err)
	c2, err := actor2.Cid()
	assert.NoError(err)

	assert.NotEqual(c1.String(), c2.String())
}

func TestActorMemory(t *testing.T) {
	assert := assert.New(t)
	actor := NewActorWithMemory(AccountActorCid, []byte{1, 2, 3})

	assert.Equal(actor.ReadStorage(), []byte{1, 2, 3})
	// write at the beginning
	actor.WriteStorage([]byte{5, 2})
	assert.Equal(actor.ReadStorage(), []byte{5, 2})

	// write overflow
	actor.WriteStorage([]byte{1, 2, 3, 4})
	assert.Equal(actor.ReadStorage(), []byte{1, 2, 3, 4})
}
