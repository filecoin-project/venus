package core

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

func TestTransfer(t *testing.T) {
	actor1 := types.NewActor(nil, types.NewTokenAmount(100))
	actor2 := types.NewActor(nil, types.NewTokenAmount(50))
	actor3 := types.NewActor(nil, nil)

	t.Run("success", func(t *testing.T) {
		assert := assert.New(t)

		assert.NoError(transfer(actor1, actor2, types.NewTokenAmount(10)))
		assert.Equal(actor1.Balance, types.NewTokenAmount(90))
		assert.Equal(actor2.Balance, types.NewTokenAmount(60))

		assert.NoError(transfer(actor1, actor3, types.NewTokenAmount(20)))
		assert.Equal(actor1.Balance, types.NewTokenAmount(70))
		assert.Equal(actor3.Balance, types.NewTokenAmount(20))
	})

	t.Run("fail", func(t *testing.T) {
		assert := assert.New(t)

		negval := types.NewTokenAmount(0).Sub(types.NewTokenAmount(1000))
		assert.EqualError(transfer(actor2, actor3, types.NewTokenAmount(1000)), "not enough balance")
		assert.EqualError(transfer(actor2, actor3, negval), "cannot transfer negative values")
	})
}
