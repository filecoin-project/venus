package types_test

import (
	"fmt"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-state-types/abi"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
)

func TestActorFormat(t *testing.T) {
	tf.UnitTest(t)

	accountActor := types.NewActor(builtin2.AccountActorCodeID, abi.NewTokenAmount(5), cid.Undef)

	formatted := fmt.Sprintf("%v", accountActor)
	assert.Contains(t, formatted, "account")
	assert.Contains(t, formatted, "balance: 5")
	assert.Contains(t, formatted, "nonce: 0")

	minerActor := types.NewActor(builtin2.StorageMinerActorCodeID, abi.NewTokenAmount(5), cid.Undef)
	formatted = fmt.Sprintf("%v", minerActor)
	assert.Contains(t, formatted, "miner")

	storageMarketActor := types.NewActor(builtin2.StorageMarketActorCodeID, abi.NewTokenAmount(5), cid.Undef)
	formatted = fmt.Sprintf("%v", storageMarketActor)
	assert.Contains(t, formatted, "market")
}
