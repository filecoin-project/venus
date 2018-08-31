package testhelpers

import (
	"context"

	"gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	"gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

// Config is used to configure values in the GenesisInitFunction
type Config struct {
	accounts map[address.Address]*types.AttoFIL
	nonces   map[address.Address]uint64
}

// GenOption is a configuration option for the GenesisInitFunction
type GenOption func(*Config) error

// ActorAccount returns a config option that sets up an actor account
func ActorAccount(addr address.Address, amt *types.AttoFIL) GenOption {
	return func(gc *Config) error {
		gc.accounts[addr] = amt
		return nil
	}
}

// ActorNonce returns a config option that sets the nonce of an existing actor
func ActorNonce(addr address.Address, nonce uint64) GenOption {
	return func(gc *Config) error {
		gc.nonces[addr] = nonce
		return nil
	}
}

// NewEmptyConfig inits and returns an empty config
func NewEmptyConfig() *Config {
	genCfg := &Config{}
	genCfg.accounts = make(map[address.Address]*types.AttoFIL)
	genCfg.nonces = make(map[address.Address]uint64)
	return genCfg
}

// MakeGenesisFunc is a method used to define a custom genesis function
func MakeGenesisFunc(opts ...GenOption) func(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*types.Block, error) {
	gif := func(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*types.Block, error) {
		genCfg := NewEmptyConfig()
		for _, opt := range opts {
			opt(genCfg) // nolint: errcheck
		}

		ctx := context.Background()
		st := state.NewEmptyStateTree(cst)
		storageMap := vm.NewStorageMap(bs)

		// Initialize account actors
		for addr, val := range genCfg.accounts {
			a, err := account.NewActor(val)
			if err != nil {
				return nil, err
			}

			if err := st.SetActor(ctx, addr, a); err != nil {
				return nil, err
			}
		}
		for addr, nonce := range genCfg.nonces {
			a, err := st.GetActor(ctx, addr)
			if err != nil {
				return nil, err
			}
			a.Nonce = types.Uint64(nonce)
			if err := st.SetActor(ctx, addr, a); err != nil {
				return nil, err
			}
		}

		// Create NetworkAddress
		a, err := account.NewActor(types.NewAttoFILFromFIL(10000000))
		if err != nil {
			return nil, err
		}
		if err := st.SetActor(ctx, address.NetworkAddress, a); err != nil {
			return nil, err
		}

		// Create StorageMarketActor
		stAct := actor.NewActor(types.StorageMarketActorCodeCid, types.NewZeroAttoFIL())
		storage := storageMap.NewStorage(address.StorageMarketAddress, stAct)
		err = (&storagemarket.Actor{}).InitializeState(storage, nil)
		if err != nil {
			return nil, err
		}

		if err := st.SetActor(ctx, address.StorageMarketAddress, stAct); err != nil {
			return nil, err
		}

		// Create PaymentBrokerActor
		pbAct := actor.NewActor(types.PaymentBrokerActorCodeCid, types.NewZeroAttoFIL())
		storage = storageMap.NewStorage(address.PaymentBrokerAddress, pbAct)
		err = (&paymentbroker.Actor{}).InitializeState(storage, nil)
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

		err = storageMap.Flush()
		if err != nil {
			return nil, err
		}

		return genesis, nil
	}

	// Pronounced "Jif" - JenesisInitFunction
	return gif
}
