package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActorMarshal(t *testing.T) {
	assert := assert.New(t)
	actor := NewActorWithMemory(AccountActorCodeCid, NewTokenAmount(1), []byte{1, 2, 3})
	actor.IncNonce()

	marshalled, err := actor.Marshal()
	assert.NoError(err)

	actorBack := Actor{}
	err = actorBack.Unmarshal(marshalled)
	assert.NoError(err)

	assert.Equal(actor.Code, actorBack.Code)
	assert.Equal(actor.ReadStorage(), actorBack.ReadStorage())
	assert.Equal(actor.Nonce, actorBack.Nonce)

	c1, err := actor.Cid()
	assert.NoError(err)
	c2, err := actorBack.Cid()
	assert.NoError(err)
	assert.Equal(c1, c2)
}

func TestActorCid(t *testing.T) {
	assert := assert.New(t)

	actor1 := NewActor(AccountActorCodeCid, nil)
	actor2 := NewActorWithMemory(AccountActorCodeCid, NewTokenAmount(5), []byte{1, 2, 3})
	actor1.IncNonce()

	c1, err := actor1.Cid()
	assert.NoError(err)
	c2, err := actor2.Cid()
	assert.NoError(err)

	assert.NotEqual(c1.String(), c2.String())
}

func TestActorMemory(t *testing.T) {
	assert := assert.New(t)
	actor := NewActorWithMemory(AccountActorCodeCid, NewTokenAmount(5), []byte{1, 2, 3})

	assert.Equal(actor.ReadStorage(), []byte{1, 2, 3})
	// write at the beginning
	actor.WriteStorage([]byte{5, 2})
	assert.Equal(actor.ReadStorage(), []byte{5, 2})

	// write overflow
	actor.WriteStorage([]byte{1, 2, 3, 4})
	assert.Equal(actor.ReadStorage(), []byte{1, 2, 3, 4})
}

func TestActorFormat(t *testing.T) {
	assert := assert.New(t)
	accountActor := NewActorWithMemory(AccountActorCodeCid, NewTokenAmount(5), []byte{1, 2, 3})

	formatted := fmt.Sprintf("%v", accountActor)
	assert.Contains(formatted, "AccountActor")
	assert.Contains(formatted, "balance: 5")
	assert.Contains(formatted, "nonce: 0")

	minerActor := NewActorWithMemory(MinerActorCodeCid, NewTokenAmount(5), []byte{1, 2, 3})
	formatted = fmt.Sprintf("%v", minerActor)
	assert.Contains(formatted, "MinerActor")

	storageMarketActor := NewActorWithMemory(StorageMarketActorCodeCid, NewTokenAmount(5), []byte{1, 2, 3})
	formatted = fmt.Sprintf("%v", storageMarketActor)
	assert.Contains(formatted, "StorageMarketActor")

	paymentBrokerActor := NewActorWithMemory(PaymentBrokerActorCodeCid, NewTokenAmount(5), []byte{1, 2, 3})
	formatted = fmt.Sprintf("%v", paymentBrokerActor)
	assert.Contains(formatted, "PaymentBrokerActor")
}
