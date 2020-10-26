package actor_test

import (
	"fmt"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"

	. "github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestActorFormat(t *testing.T) {
	tf.UnitTest(t)

	accountActor := NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(5), cid.Undef)

	formatted := fmt.Sprintf("%v", accountActor)
	assert.Contains(t, formatted, "account")
	assert.Contains(t, formatted, "balance: 5")
	assert.Contains(t, formatted, "nonce: 0")

	minerActor := NewActor(builtin.StorageMinerActorCodeID, abi.NewTokenAmount(5), cid.Undef)
	formatted = fmt.Sprintf("%v", minerActor)
	assert.Contains(t, formatted, "miner")

	storageMarketActor := NewActor(builtin.StorageMarketActorCodeID, abi.NewTokenAmount(5), cid.Undef)
	formatted = fmt.Sprintf("%v", storageMarketActor)
	assert.Contains(t, formatted, "market")
}
