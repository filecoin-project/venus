package porcelain_test

import (
	"context"
	"testing"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/types"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

type walletTestPlumbing struct {
	balance *types.AttoFIL
	addr    address.Address
}

func (wtp *walletTestPlumbing) ActorGet(ctx context.Context, addr address.Address) (*actor.Actor, error) {
	testActor := actor.NewActor(cid.Undef, wtp.balance)
	return testActor, nil
}

func (wtp *walletTestPlumbing) ConfigGet(dottedPath string) (interface{}, error) {
	testHash := address.Hash([]byte("test"))
	return address.NewMainnet(testHash), nil

}

func TestWalletBalance(t *testing.T) {
	t.Run("Returns the correct value for wallet balance", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		ctx := context.Background()

		expectedBalance := types.NewAttoFILFromFIL(20)
		plumbing := &walletTestPlumbing{
			balance: expectedBalance,
		}
		balance, err := porcelain.WalletBalance(ctx, plumbing, address.Address{})
		require.NoError(err)

		assert.Equal(expectedBalance, balance)
	})
}

func TestDefaultWalletAddress(t *testing.T) {
	t.Run("Returns the correct value for default wallet address", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		testHash := address.Hash([]byte("test"))
		expectedAddress := address.NewMainnet(testHash)
		plumbing := &walletTestPlumbing{
			addr: expectedAddress,
		}
		address, err := porcelain.DefaultWalletAddress(plumbing)
		require.NoError(err)

		assert.Equal(expectedAddress, address)
	})
}
