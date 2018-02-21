package core

import (
	"math/big"
	"testing"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

func TestTransfer(t *testing.T) {
	actor1 := types.NewActor(nil, big.NewInt(100))
	actor2 := types.NewActor(nil, big.NewInt(50))
	actor3 := types.NewActor(nil, nil)

	t.Run("success", func(t *testing.T) {
		assert := assert.New(t)

		assert.NoError(transfer(actor1, actor2, big.NewInt(10)))
		assert.Equal(actor1.Balance, big.NewInt(90))
		assert.Equal(actor2.Balance, big.NewInt(60))

		assert.NoError(transfer(actor1, actor3, big.NewInt(20)))
		assert.Equal(actor1.Balance, big.NewInt(70))
		assert.Equal(actor3.Balance, big.NewInt(20))
	})

	t.Run("fail", func(t *testing.T) {
		assert := assert.New(t)

		assert.EqualError(transfer(actor2, actor3, big.NewInt(1000)), "from has not enough balance")
		assert.EqualError(transfer(actor2, actor3, big.NewInt(-1000)), "can not transfer negative values")
	})
}
