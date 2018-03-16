package core

import (
	"context"
	"math/big"

	"gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/types"
)

// GenesisInitFunc is the signature for function that is used to create a genesis block.
type GenesisInitFunc func(cst *hamt.CborIpldStore) (*types.Block, error)

var (
	// TestAccount is an account with some initial funds in it
	TestAccount types.Address
	// NetworkAccount is the filecoin network
	NetworkAccount types.Address
	// StorageMarketAddress is the hard-coded address of the filecoin storage market
	StorageMarketAddress types.Address

	defaultAccounts map[types.Address]int64
)

func init() {
	t, err := types.AddressHash([]byte("satoshi"))
	if err != nil {
		panic(err)
	}
	TestAccount = types.NewMainnetAddress(t)

	n, err := types.AddressHash([]byte("filecoin"))
	if err != nil {
		panic(err)
	}
	NetworkAccount = types.NewMainnetAddress(n)

	s, err := types.AddressHash([]byte("storage"))
	if err != nil {
		panic(err)
	}

	StorageMarketAddress = types.NewMainnetAddress(s)

	defaultAccounts = map[types.Address]int64{
		NetworkAccount: 10000000,
		TestAccount:    50000,
	}
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

		if err := st.SetActor(ctx, addr, a); err != nil {
			return nil, err
		}
	}

	stAct, err := NewStorageMarketActor()
	if err != nil {
		return nil, err
	}
	if err := st.SetActor(ctx, StorageMarketAddress, stAct); err != nil {
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
