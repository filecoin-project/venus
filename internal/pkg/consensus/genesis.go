package consensus

import (
	"context"

	"github.com/filecoin-project/go-amt-ipld"
	"github.com/filecoin-project/go-bls-sigs"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// GenesisInitFunc is the signature for function that is used to create a genesis block.
type GenesisInitFunc func(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*block.Block, error)

var (
	defaultAccounts map[address.Address]types.AttoFIL
)

func init() {
	defaultAccounts = map[address.Address]types.AttoFIL{
		address.NetworkAddress:    types.NewAttoFILFromFIL(10000000000),
		address.BurntFundsAddress: types.NewAttoFILFromFIL(0),
		address.TestAddress:       types.NewAttoFILFromFIL(50000),
		address.TestAddress2:      types.NewAttoFILFromFIL(60000),
	}
}

type minerActorConfig struct {
	state   *miner.State
	balance types.AttoFIL
}

// Config is used to configure values in the GenesisInitFunction.
type Config struct {
	accounts   map[address.Address]types.AttoFIL
	nonces     map[address.Address]uint64
	actors     map[address.Address]*actor.Actor
	miners     map[address.Address]*minerActorConfig
	network    string
	proofsMode types.ProofsMode
}

// GenOption is a configuration option for the GenesisInitFunction.
type GenOption func(*Config) error

// ActorAccount returns a config option that sets up an actor account.
func ActorAccount(addr address.Address, amt types.AttoFIL) GenOption {
	return func(gc *Config) error {
		gc.accounts[addr] = amt
		return nil
	}
}

// MinerActor returns a config option that sets up an miner actor account.
func MinerActor(addr address.Address, owner address.Address, pid peer.ID, coll types.AttoFIL, sectorSize *types.BytesAmount) GenOption {
	return func(gc *Config) error {
		gc.miners[addr] = &minerActorConfig{
			state:   miner.NewState(owner, owner, pid, sectorSize),
			balance: coll,
		}
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

// AddActor returns a config option that sets an arbitrary actor. You
// will need to set the mapping from the actor's codecid to implementation
// in builtin.Actors if it is not there already.
func AddActor(addr address.Address, actor *actor.Actor) GenOption {
	return func(gc *Config) error {
		gc.actors[addr] = actor
		return nil
	}
}

// NetworkName returns a config option that sets the network name.
func NetworkName(name string) GenOption {
	return func(gc *Config) error {
		gc.network = name
		return nil
	}
}

// Network sets the network name for the network created by the genesis node.
func Network(name string) GenOption {
	return func(gc *Config) error {
		gc.network = name
		return nil
	}
}

// ProofsMode sets the mode of operation for the proofs library.
func ProofsMode(proofsMode types.ProofsMode) GenOption {
	return func(gc *Config) error {
		gc.proofsMode = proofsMode
		return nil
	}
}

// NewEmptyConfig inits and returns an empty config
func NewEmptyConfig() *Config {
	return &Config{
		accounts:   make(map[address.Address]types.AttoFIL),
		nonces:     make(map[address.Address]uint64),
		actors:     make(map[address.Address]*actor.Actor),
		miners:     make(map[address.Address]*minerActorConfig),
		network:    "localnet",
		proofsMode: types.TestProofsMode,
	}
}

// MakeGenesisFunc returns a genesis function configured by a set of options.
func MakeGenesisFunc(opts ...GenOption) GenesisInitFunc {
	return func(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*block.Block, error) {
		ctx := context.Background()
		st := state.NewEmptyStateTree(cst)
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
		// Initialize miner actors
		for addr, val := range genCfg.miners {
			a := miner.NewActor()
			a.Balance = val.balance
			s := storageMap.NewStorage(addr, a)
			scid, err := s.Put(val.state)
			if err != nil {
				return nil, err
			}
			if err = s.Commit(scid, a.Head); err != nil {
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
		if err := SetupDefaultActors(ctx, st, storageMap, genCfg.proofsMode, genCfg.network); err != nil {
			return nil, err
		}
		// Now add any other actors configured.
		for addr, a := range genCfg.actors {
			if err := st.SetActor(ctx, addr, a); err != nil {
				return nil, err
			}
		}

		if err := actor.InitBuiltinActorCodeObjs(cst); err != nil {
			return nil, err
		}

		c, err := st.Flush(ctx)
		if err != nil {
			return nil, err
		}

		emptyAMTCid, err := amt.FromArray(amt.WrapBlockstore(bs), []typegen.CBORMarshaler{})
		if err != nil {
			return nil, err
		}

		emptyBLSSignature := bls.Aggregate([]bls.Signature{})

		genesis := &block.Block{
			StateRoot:       c,
			Messages:        types.TxMeta{SecpRoot: emptyAMTCid, BLSRoot: emptyAMTCid},
			MessageReceipts: emptyAMTCid,
			BLSAggregateSig: emptyBLSSignature[:],
			Ticket:          block.Ticket{VRFProof: []byte{0xec}},
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

// SetupDefaultActors inits the builtin actors that are required to run filecoin.
func SetupDefaultActors(ctx context.Context, st state.Tree, storageMap vm.StorageMap, storeType types.ProofsMode, network string) error {
	for addr, val := range defaultAccounts {
		a, err := account.NewActor(val)
		if err != nil {
			return err
		}

		if err := st.SetActor(ctx, addr, a); err != nil {
			return err
		}
	}

	stAct := storagemarket.NewActor()
	err := (&storagemarket.Actor{}).InitializeState(storageMap.NewStorage(address.StorageMarketAddress, stAct), storeType)
	if err != nil {
		return err
	}
	if err := st.SetActor(ctx, address.StorageMarketAddress, stAct); err != nil {
		return err
	}

	pbAct := paymentbroker.NewActor()
	err = (&paymentbroker.Actor{}).InitializeState(storageMap.NewStorage(address.PaymentBrokerAddress, pbAct), nil)
	if err != nil {
		return err
	}
	if err = st.SetActor(ctx, address.PaymentBrokerAddress, pbAct); err != nil {
		return err
	}

	intAct := initactor.NewActor()
	err = (&initactor.Actor{}).InitializeState(storageMap.NewStorage(address.InitAddress, intAct), network)
	if err != nil {
		return err
	}
	return st.SetActor(ctx, address.InitAddress, intAct)
}
