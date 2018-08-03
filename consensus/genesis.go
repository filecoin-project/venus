package consensus

import (
	"context"

	"gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
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
	defaultAccounts map[address.Address]*types.AttoFIL
)

func init() {
	defaultAccounts = map[address.Address]*types.AttoFIL{
		address.NetworkAddress: types.NewAttoFILFromFIL(10000000000),
		address.TestAddress:    types.NewAttoFILFromFIL(50000),
		address.TestAddress2:   types.NewAttoFILFromFIL(60000),
	}
}

// Config is used to configure values in the GenesisInitFunction.
type Config struct {
	accounts map[address.Address]*types.AttoFIL
	nonces   map[address.Address]uint64
}

// GenOption is a configuration option for the GenesisInitFunction.
type GenOption func(*Config) error

// ActorAccount returns a config option that sets up an actor account.
func ActorAccount(addr address.Address, amt *types.AttoFIL) GenOption {
	return func(gc *Config) error {
		gc.accounts[addr] = amt
		return nil
	}
}

// ActorNonce returns a config option that sets the nonce of an existing actor.
func ActorNonce(addr address.Address, nonce uint64) GenOption {
	return func(gc *Config) error {
		gc.nonces[addr] = nonce
		return nil
	}
}

// NewEmptyConfig inits and returns an empty config
func NewEmptyConfig() *Config {
	return &Config{
		accounts: make(map[address.Address]*types.AttoFIL),
		nonces:   make(map[address.Address]uint64),
	}
}

// MakeGenesisFunc is a method used to define a custom genesis function
func MakeGenesisFunc(opts ...GenOption) func(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*types.Block, error) {
	return func(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*types.Block, error) {
		ctx := context.Background()
		st := state.NewEmptyStateTreeWithActors(cst, builtin.Actors)
		storageMap := vm.NewStorageMap(bs)

		genCfg := NewEmptyConfig()
		for _, opt := range opts {
			if err := opt(genCfg); err != nil {
				return nil, err
			}
		}

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
		if err := SetupDefaultActors(ctx, st, storageMap); err != nil {
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
}

// InitGenesis is the default function to create the genesis block.
func InitGenesis(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*types.Block, error) {
	return MakeGenesisFunc()(cst, bs)
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

	stAct, err := storagemarket.NewActor()
	if err != nil {
		return err
	}
	err = (&storagemarket.Actor{}).InitializeState(storageMap.NewStorage(address.StorageMarketAddress, stAct), nil)
	if err != nil {
		return err
	}
	if err := st.SetActor(ctx, address.StorageMarketAddress, stAct); err != nil {
		return err
	}

	pbAct := actor.NewActor(types.PaymentBrokerActorCodeCid, types.NewZeroAttoFIL())
	err = (&paymentbroker.Actor{}).InitializeState(storageMap.NewStorage(address.PaymentBrokerAddress, pbAct), nil)
	if err != nil {
		return err
	}

	pbAct.Balance = types.NewAttoFILFromFIL(0)

	return st.SetActor(ctx, address.PaymentBrokerAddress, pbAct)
}

// ApplyMessageDirect applies a given message directly to the given state tree and storage map.
func ApplyMessageDirect(ctx context.Context, st state.Tree, storageMap vm.StorageMap, from, to address.Address, value *types.AttoFIL, method string, params ...interface{}) (*ApplicationResult, error) {
	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return nil, errors.Wrap(err, "invalid params")
	}

	fromActor, err := st.GetActor(ctx, from)
	if err != nil {
		return nil, errors.Wrap(err, "invalid from actor")
	}

	message := types.NewMessage(from, to, uint64(fromActor.Nonce), value, method, encodedParams)
	return ApplyMessage(ctx, st, storageMap, message, types.NewBlockHeight(0))
}
