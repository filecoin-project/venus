package types

import (
	"fmt"
	"testing"

	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActorMarshal(t *testing.T) {
	assert := assert.New(t)
	actor := NewActor(AccountActorCodeCid, NewAttoFILFromFIL(1))
	actor.Head = requireCid(t, "Actor Storage")
	actor.IncNonce()

	marshalled, err := actor.Marshal()
	assert.NoError(err)

	actorBack := Actor{}
	err = actorBack.Unmarshal(marshalled)
	assert.NoError(err)

	assert.Equal(actor.Code, actorBack.Code)
	assert.Equal(actor.Head, actorBack.Head)
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
	actor2 := NewActor(AccountActorCodeCid, NewAttoFILFromFIL(5))
	actor2.Head = requireCid(t, "Actor 2 State")
	actor1.IncNonce()

	c1, err := actor1.Cid()
	assert.NoError(err)
	c2, err := actor2.Cid()
	assert.NoError(err)

	assert.NotEqual(c1.String(), c2.String())
}

func TestActorFormat(t *testing.T) {
	assert := assert.New(t)
	accountActor := NewActor(AccountActorCodeCid, NewAttoFILFromFIL(5))

	formatted := fmt.Sprintf("%v", accountActor)
	assert.Contains(formatted, "AccountActor")
	assert.Contains(formatted, "balance: 5")
	assert.Contains(formatted, "nonce: 0")

	minerActor := NewActor(MinerActorCodeCid, NewAttoFILFromFIL(5))
	formatted = fmt.Sprintf("%v", minerActor)
	assert.Contains(formatted, "MinerActor")

	storageMarketActor := NewActor(StorageMarketActorCodeCid, NewAttoFILFromFIL(5))
	formatted = fmt.Sprintf("%v", storageMarketActor)
	assert.Contains(formatted, "StorageMarketActor")

	paymentBrokerActor := NewActor(PaymentBrokerActorCodeCid, NewAttoFILFromFIL(5))
	formatted = fmt.Sprintf("%v", paymentBrokerActor)
	assert.Contains(formatted, "PaymentBrokerActor")
}

func requireCid(t *testing.T, data string) *cid.Cid {
	prefix := cid.NewPrefixV1(cid.Raw, DefaultHashFunction)
	cid, err := prefix.Sum([]byte(data))
	require.NoError(t, err)
	return cid
}
