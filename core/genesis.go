package core

import (
	"context"
	"math/big"

	"gx/ipfs/QmdBXcN47jVwKLwSyN9e9xYVZ7WcAWgQ5N4cmNw7nzWq2q/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/types"
)

// GenesisInitFunc is the signature for function that is used to create a genesis block.
type GenesisInitFunc func(cst *hamt.CborIpldStore) (*types.Block, error)

var defaultAccounts = map[string]int64{
	// the filecoin network
	"filecoin": 100000,
	// some lucky investor, used in testing and bootstraping for now
	"investor1": 500,
}

// InitGenesis is the default function to create the genesis block.
func InitGenesis(cst *hamt.CborIpldStore) (*types.Block, error) {
	ctx := context.Background()
	st := types.NewEmptyStateTree(cst)

	balances := map[types.Address]*Balance{}
	for addr, val := range defaultAccounts {
		a, err := NewAccountActor()
		if err != nil {
			return nil, err
		}
		balances[types.Address(addr)] = &Balance{Total: big.NewInt(val)}

		if err := st.SetActor(ctx, types.Address(addr), a); err != nil {
			return nil, err
		}
	}

	t, err := NewTokenActor(balances)
	if err != nil {
		return nil, err
	}

	if err := st.SetActor(ctx, types.Address("token"), t); err != nil {
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
