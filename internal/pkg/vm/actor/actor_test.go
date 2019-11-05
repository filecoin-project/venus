package actor_test

import (
	"fmt"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	. "github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestActorCid(t *testing.T) {
	tf.UnitTest(t)

	actor1 := NewActor(types.AccountActorCodeCid, types.ZeroAttoFIL)
	actor2 := NewActor(types.AccountActorCodeCid, types.NewAttoFILFromFIL(5))
	actor2.Head = requireCid(t, "Actor 2 State")
	actor1.IncNonce()

	c1, err := actor1.Cid()
	assert.NoError(t, err)
	c2, err := actor2.Cid()
	assert.NoError(t, err)

	assert.NotEqual(t, c1.String(), c2.String())
}

func TestActorFormat(t *testing.T) {
	tf.UnitTest(t)

	accountActor := NewActor(types.AccountActorCodeCid, types.NewAttoFILFromFIL(5))

	formatted := fmt.Sprintf("%v", accountActor)
	assert.Contains(t, formatted, "AccountActor")
	assert.Contains(t, formatted, "balance: 5")
	assert.Contains(t, formatted, "nonce: 0")

	minerActor := NewActor(types.MinerActorCodeCid, types.NewAttoFILFromFIL(5))
	formatted = fmt.Sprintf("%v", minerActor)
	assert.Contains(t, formatted, "MinerActor")

	storageMarketActor := NewActor(types.StorageMarketActorCodeCid, types.NewAttoFILFromFIL(5))
	formatted = fmt.Sprintf("%v", storageMarketActor)
	assert.Contains(t, formatted, "StorageMarketActor")

	paymentBrokerActor := NewActor(types.PaymentBrokerActorCodeCid, types.NewAttoFILFromFIL(5))
	formatted = fmt.Sprintf("%v", paymentBrokerActor)
	assert.Contains(t, formatted, "PaymentBrokerActor")
}

func requireCid(t *testing.T, data string) cid.Cid {
	prefix := cid.V1Builder{Codec: cid.Raw, MhType: types.DefaultHashFunction}
	cid, err := prefix.Sum([]byte(data))
	require.NoError(t, err)
	return cid
}
