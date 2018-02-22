package core

import (
	"context"
	"math/big"

	"gx/ipfs/QmZhoiN2zi5SBBBKb181dQm4QdvWAvEwbppZvKpp4gRyNY/go-hamt-ipld"

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

	for addr, val := range defaultAccounts {
		a, err := NewAccountActor(big.NewInt(val))
		if err != nil {
			return nil, err
		}

		if err := st.SetActor(ctx, types.Address(addr), a); err != nil {
			return nil, err
		}
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
