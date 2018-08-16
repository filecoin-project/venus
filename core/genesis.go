package core

import (
	"context"

	"gx/ipfs/QmbwwhSsEcSPP4XfGumu6GMcuCLnCLVQAnp3mDxKuYNXJo/go-hamt-ipld"
	"gx/ipfs/QmcD7SqfyQyA91TZUQ7VPRYbGarxmY7EsQewVYMuN5LNSv/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

// GenesisInitFunc is the signature for function that is used to create a genesis block.
type GenesisInitFunc func(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*types.Block, error)

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
func InitGenesis(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*types.Block, error) {
	ctx := context.Background()
	st := state.NewEmptyStateTree(cst)
	storageMap := vm.NewStorageMap(bs)

	if err := SetupDefaultActors(ctx, st, storageMap); err != nil {
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

// SetupDefaultActors inits the builtin actors that are required to run filecoin.
func SetupDefaultActors(ctx context.Context, st state.Tree, storageMap vm.StorageMap) error {
	for addr, val := range defaultAccounts {
		a, err := account.NewActor(val)
		if err != nil {
			return err
		}

		if err := st.SetActor(ctx, addr, a); err != nil {
			return err
		}
	}

	stAct := types.NewActor(types.StorageMarketActorCodeCid, types.NewZeroAttoFIL())
	err := (&storagemarket.Actor{}).InitializeState(storageMap.NewStorage(address.StorageMarketAddress, stAct), nil)
	if err != nil {
		return err
	}
	if err := st.SetActor(ctx, address.StorageMarketAddress, stAct); err != nil {
		return err
	}

	pbAct := types.NewActor(types.PaymentBrokerActorCodeCid, types.NewZeroAttoFIL())
	err = (&paymentbroker.Actor{}).InitializeState(storageMap.NewStorage(address.PaymentBrokerAddress, pbAct), nil)
	if err != nil {
		return err
	}

	pbAct.Balance = types.NewAttoFILFromFIL(0)

	return st.SetActor(ctx, address.PaymentBrokerAddress, pbAct)
}
