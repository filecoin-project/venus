package chain

import (
	"context"

	"gx/ipfs/QmdBXcN47jVwKLwSyN9e9xYVZ7WcAWgQ5N4cmNw7nzWq2q/go-hamt-ipld"

	state "github.com/filecoin-project/go-filecoin/state"
	types "github.com/filecoin-project/go-filecoin/types"
)

type GenesisInitFunc func(cst *hamt.CborIpldStore) (*types.Block, error)

func InitGenesis(cst *hamt.CborIpldStore) (*types.Block, error) {
	st := state.NewEmptyTree(cst)

	c, err := st.Flush(context.Background())
	if err != nil {
		return nil, err
	}

	genesis := &types.Block{
		StateRoot: c,
		Nonce:     1337,
	}

	if _, err := cst.Put(context.Background(), genesis); err != nil {
		return nil, err
	}

	return genesis, nil
}
