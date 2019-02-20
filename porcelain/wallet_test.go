package porcelain_test

import (
	"context"
	"testing"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type walletTestPlumbing struct{}

func (wtp *walletTestPlumbing) ChainLatestState(ctx context.Context) (state.Tree, error) {
	store := hamt.NewCborStore()
	tree := state.NewEmptyStateTree(store)

	testActor := actor.NewActor(cid.Undef, types.NewAttoFILFromFIL(20))
	tree.SetActor(ctx, address.Address{}, testActor)

	return tree, nil
}

func TestWalletBalance(t *testing.T) {
	t.Run("Returns the correct value for wallet balance", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		ctx := context.Background()

		balance, err := porcelain.WalletBalance(ctx, &walletTestPlumbing{}, address.Address{})
		require.NoError(err)

		assert.Equal(types.NewAttoFILFromFIL(20), balance)
	})
}
