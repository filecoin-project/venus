package core

import (
	"context"

	hamt "gx/ipfs/QmXJkSRxXHeAGmQJENct16anrKZHNECbmUoC7hMuCjLni6/go-hamt-ipld"
	"gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"

	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

// GenesisInitFunc is the signature for function that is used to create a genesis block.
type GenesisInitFunc func(cst *hamt.CborIpldStore, ds datastore.Datastore) (*types.Block, error)

var (
	defaultAccounts map[types.Address]*types.AttoFIL
)

func init() {
	defaultAccounts = map[types.Address]*types.AttoFIL{
		address.NetworkAddress: types.NewAttoFILFromFIL(10000000),
		address.TestAddress:    types.NewAttoFILFromFIL(50000),
		address.TestAddress2:   types.NewAttoFILFromFIL(60000),
	}
}

// InitGenesis is the default function to create the genesis block.
func InitGenesis(cst *hamt.CborIpldStore, ds datastore.Datastore) (*types.Block, error) {
	ctx := context.Background()
	st := state.NewEmptyStateTree(cst)
	storageMap := vm.NewStorageMap(ds)

	for addr, val := range defaultAccounts {
		a, err := account.NewActor(val)
		if err != nil {
			return nil, err
		}

		if err := st.SetActor(ctx, addr, a); err != nil {
			return nil, err
		}
	}

	stAct := types.NewActor(types.StorageMarketActorCodeCid, types.NewZeroAttoFIL())
	err := storagemarket.InitializeState(storageMap.NewStorage(address.StorageMarketAddress, stAct), nil)
	if err != nil {
		return nil, err
	}
	if err := st.SetActor(ctx, address.StorageMarketAddress, stAct); err != nil {
		return nil, err
	}

	pbAct := types.NewActor(types.PaymentBrokerActorCodeCid, types.NewZeroAttoFIL())
	err = paymentbroker.InitializeState(storageMap.NewStorage(address.PaymentBrokerAddress, pbAct), nil)
	pbAct.Balance = types.NewAttoFILFromFIL(0)
	if err != nil {
		return nil, err
	}
	if err := st.SetActor(ctx, address.PaymentBrokerAddress, pbAct); err != nil {
		return nil, err
	}

	c, err := st.Flush(ctx)
	if err != nil {
		return nil, err
	}

	err = storageMap.Flush()
	if err != nil {
		return nil, err
	}

	genesis := &types.Block{
		StateRoot: c,
		Nonce:     1337,
	}

	if _, err := cst.Put(ctx, genesis); err != nil {
		return nil, err
	}

	return genesis, nil
}
