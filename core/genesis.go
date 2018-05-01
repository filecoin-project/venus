package core

import (
	"context"

	"gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

// GenesisInitFunc is the signature for function that is used to create a genesis block.
type GenesisInitFunc func(cst *hamt.CborIpldStore) (*types.Block, error)

var (
	defaultAccounts map[types.Address]uint64
)

func init() {
	defaultAccounts = map[types.Address]uint64{
		address.NetworkAddress: 10000000,
		address.TestAddress:    50000,
		address.TestAddress2:   60000,
	}
}

// InitGenesis is the default function to create the genesis block.
func InitGenesis(cst *hamt.CborIpldStore) (*types.Block, error) {
	ctx := context.Background()
	st := state.NewEmptyStateTree(cst)

	for addr, val := range defaultAccounts {
		a, err := account.NewActor(types.NewTokenAmount(val))
		if err != nil {
			return nil, err
		}

		if err := st.SetActor(ctx, addr, a); err != nil {
			return nil, err
		}
	}

	stAct, err := storagemarket.NewActor()
	if err != nil {
		return nil, err
	}
	if err := st.SetActor(ctx, address.StorageMarketAddress, stAct); err != nil {
		return nil, err
	}

	c, err := st.Flush(ctx)
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
