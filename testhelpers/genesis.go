package testhelpers

import (
	"context"

	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"

	"gx/ipfs/QmcYBp5EDnJKfVN63F71rDTksvEf1cfijwCTWtw6bPG58T/go-hamt-ipld"
)

// Config is used to configure values in the GenesisInitFunction
type Config struct {
	accounts map[types.Address]*types.AttoFIL
}

// GenOption is a configuration option for the GenesisInitFunction
type GenOption func(*Config) error

// ActorAccount returns a config option that sets up an actor account
func ActorAccount(addr types.Address, amt *types.AttoFIL) GenOption {
	return func(gc *Config) error {
		gc.accounts[addr] = amt
		return nil
	}
}

// NewEmptyConfig inits and returns an empty config
func NewEmptyConfig() *Config {
	genCfg := &Config{}
	genCfg.accounts = make(map[types.Address]*types.AttoFIL)
	return genCfg
}

// MakeGenesisFunc is a method used to define a custom genesis function
func MakeGenesisFunc(opts ...GenOption) core.GenesisInitFunc {
	gif := func(cst *hamt.CborIpldStore) (*types.Block, error) {
		genCfg := NewEmptyConfig()
		for _, opt := range opts {
			opt(genCfg) // nolint: errcheck
		}

		ctx := context.Background()
		st := state.NewEmptyStateTree(cst)

		for addr, val := range genCfg.accounts {
			a, err := account.NewActor(val)
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

		pbAct, err := paymentbroker.NewPaymentBrokerActor()
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

		genesis := &types.Block{
			StateRoot: c,
			Nonce:     1337,
		}

		if _, err := cst.Put(ctx, genesis); err != nil {
			return nil, err
		}

		return genesis, nil
	}

	// Pronounced "Jif" - JenesisInitFunction
	return gif
}
