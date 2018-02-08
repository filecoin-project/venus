package chain

import (
	"context"
	"math/big"

	"gx/ipfs/QmdBXcN47jVwKLwSyN9e9xYVZ7WcAWgQ5N4cmNw7nzWq2q/go-hamt-ipld"

	state "github.com/filecoin-project/go-filecoin/state"
	types "github.com/filecoin-project/go-filecoin/types"
)

type GenesisInitFunc func(cst *hamt.CborIpldStore) (*types.Block, error)

func InitGenesis(cst *hamt.CborIpldStore) (*types.Block, error) {
	ctx := context.Background()
	st := state.NewEmptyTree(cst)

	filNetwork := &state.Actor{
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
