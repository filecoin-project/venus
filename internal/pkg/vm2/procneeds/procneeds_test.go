package procneeds

import (
	"testing"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestTransfer(t *testing.T) {
	tf.UnitTest(t)

	actor1 := actor.NewActor(cid.Undef, types.NewAttoFILFromFIL(100))
	actor2 := actor.NewActor(cid.Undef, types.NewAttoFILFromFIL(50))
	actor3 := actor.NewActor(cid.Undef, types.ZeroAttoFIL)

	t.Run("success", func(t *testing.T) {
		assert.NoError(t, Transfer(actor1, actor2, types.NewAttoFILFromFIL(10)))
		assert.Equal(t, actor1.Balance, types.NewAttoFILFromFIL(90))
		assert.Equal(t, actor2.Balance, types.NewAttoFILFromFIL(60))

		assert.NoError(t, Transfer(actor1, actor3, types.NewAttoFILFromFIL(20)))
		assert.Equal(t, actor1.Balance, types.NewAttoFILFromFIL(70))
		assert.Equal(t, actor3.Balance, types.NewAttoFILFromFIL(20))
	})

	t.Run("fail", func(t *testing.T) {
		negval := types.NewAttoFILFromFIL(0).Sub(types.NewAttoFILFromFIL(1000))
		assert.EqualError(t, Transfer(actor2, actor3, types.NewAttoFILFromFIL(1000)), "not enough balance")
		assert.EqualError(t, Transfer(actor2, actor3, negval), "cannot transfer negative values")
	})
}
