package core

import (
	"context"
	"math/big"

	"gx/ipfs/QmZhoiN2zi5SBBBKb181dQm4QdvWAvEwbppZvKpp4gRyNY/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/types"
)

// GenesisInitFunc is the signature for function that is used to create a genesis block.
type GenesisInitFunc func(cst *hamt.CborIpldStore) (*types.Block, error)

// InitGenesis is the default function to create the genesis block.
func InitGenesis(cst *hamt.CborIpldStore) (*types.Block, error) {
	ctx := context.Background()
	st := types.NewEmptyStateTree(cst)

	filNetwork := &types.Actor{
		Balance: big.NewInt(1000000000),
	}

	if err := st.SetActor(ctx, types.Address("filecoin"), filNetwork); err != nil {
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
