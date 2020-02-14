package actor_test

import (
	"fmt"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	. "github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestActorCid(t *testing.T) {
	tf.UnitTest(t)

	actor1 := NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(0))
	actor2 := NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(5))
	actor2.Head = e.NewCid(requireCid(t, "Actor 2 State"))
	actor1.IncrementSeqNum()

	c1, err := actor1.Cid()
	assert.NoError(t, err)
	c2, err := actor2.Cid()
	assert.NoError(t, err)

	assert.NotEqual(t, c1.String(), c2.String())
}

func TestActorFormat(t *testing.T) {
	tf.UnitTest(t)

	accountActor := NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(5))

	formatted := fmt.Sprintf("%v", accountActor)
	assert.Contains(t, formatted, "account")
	assert.Contains(t, formatted, "balance: 5")
	assert.Contains(t, formatted, "nonce: 0")

	minerActor := NewActor(builtin.StorageMinerActorCodeID, abi.NewTokenAmount(5))
	formatted = fmt.Sprintf("%v", minerActor)
	assert.Contains(t, formatted, "miner")

	storageMarketActor := NewActor(builtin.StorageMarketActorCodeID, abi.NewTokenAmount(5))
	formatted = fmt.Sprintf("%v", storageMarketActor)
	assert.Contains(t, formatted, "market")
}

func requireCid(t *testing.T, data string) cid.Cid {
	prefix := cid.V1Builder{Codec: cid.Raw, MhType: types.DefaultHashFunction}
	cid, err := prefix.Sum([]byte(data))
	require.NoError(t, err)
	return cid
}
