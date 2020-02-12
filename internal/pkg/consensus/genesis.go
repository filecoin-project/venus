package consensus

import (
	"context"
	"sort"
	"time"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-amt-ipld/v2"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"
	typegen "github.com/whyrusleeping/cbor-gen"

	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/power"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/storagemarket"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// GenesisInitFunc is the signature for function that is used to create a genesis block.
type GenesisInitFunc func(cst cbor.IpldStore, bs blockstore.Blockstore) (*block.Block, error)

var (
	defaultAccounts map[address.Address]abi.TokenAmount
)

func init() {
	defaultAccounts = map[address.Address]abi.TokenAmount{
		vmaddr.LegacyNetworkAddress: abi.NewTokenAmount(10000000000),
		vmaddr.BurntFundsAddress:    abi.NewTokenAmount(0),
		vmaddr.TestAddress:          abi.NewTokenAmount(50000),
		vmaddr.TestAddress2:         abi.NewTokenAmount(60000),
	}
}

type minerActorConfig struct {
	state   *miner.State
	balance abi.TokenAmount
}

// Config is used to configure values in the GenesisInitFunction.
type Config struct {
	accounts         map[address.Address]abi.TokenAmount
	nonces           map[address.Address]uint64
	actors           map[address.Address]*actor.Actor
	miners           map[address.Address]*minerActorConfig
	network          string
	proofsMode       types.ProofsMode
	genesisTimestamp time.Time
}

// GenOption is a configuration option for the GenesisInitFunction.
type GenOption func(*Config) error

// ActorAccount returns a config option that sets up an actor account.
func ActorAccount(addr address.Address, amt abi.TokenAmount) GenOption {
	return func(gc *Config) error {
		gc.accounts[addr] = amt
		return nil
	}
}

// MinerActor returns a config option that sets up an miner actor account.
func MinerActor(addr address.Address, owner address.Address, pid peer.ID, coll abi.TokenAmount, sectorSize *types.BytesAmount) GenOption {
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

// GenesisTime sets the timestamp in the genesis block.
func GenesisTime(genesisTimestamp time.Time) GenOption {
	return func(gc *Config) error {
		gc.genesisTimestamp = genesisTimestamp
		return nil
	}
}

var defaultGenesisTimestamp = time.Unix(123456789, 0)

// NewEmptyConfig inits and returns an empty config
func NewEmptyConfig() *Config {
	return &Config{
		accounts:         make(map[address.Address]abi.TokenAmount),
		nonces:           make(map[address.Address]uint64),
		actors:           make(map[address.Address]*actor.Actor),
		miners:           make(map[address.Address]*minerActorConfig),
		network:          "localnet",
		proofsMode:       types.TestProofsMode,
		genesisTimestamp: defaultGenesisTimestamp,
	}
}

// GenesisVM is the view into the VM used during genesis block creation.
type GenesisVM interface {
	ApplyGenesisMessage(from address.Address, to address.Address, method types.MethodID, value abi.TokenAmount, params interface{}) (interface{}, error)
}

// MakeGenesisFunc returns a genesis function configured by a set of options.
func MakeGenesisFunc(opts ...GenOption) GenesisInitFunc {
	return func(cst cbor.IpldStore, bs blockstore.Blockstore) (*block.Block, error) {
		ctx := context.Background()
		st := state.NewTree(cst)
		store := vm.NewStorage(bs)
		vm := vm.NewVM(st, &store).(GenesisVM)

		genCfg := NewEmptyConfig()
		for _, opt := range opts {
			if err := opt(genCfg); err != nil {
				return nil, err
			}
		}

		if err := SetupDefaultActors(ctx, vm, &store, st, genCfg.proofsMode, genCfg.network); err != nil {
			return nil, err
		}

		// sort addresses so genesis generation will be stable
		sortedAddresses := []string{}
		for addr := range genCfg.accounts {
			sortedAddresses = append(sortedAddresses, string(addr.Bytes()))
		}
		sort.Strings(sortedAddresses)

		// Initialize account actors
		for _, addrStr := range sortedAddresses {
			addr, err := address.NewFromBytes([]byte(addrStr))
			if err != nil {
				return nil, err
			}
			val := genCfg.accounts[addr]

			constructorParams, err := encoding.Encode(addr)
			if err != nil {
				return nil, err
			}
			_, err = vm.ApplyGenesisMessage(vmaddr.LegacyNetworkAddress, vmaddr.InitAddress,
				initactor.ExecMethodID, val, initactor.ExecParams{
					ActorCodeCid:      types.AccountActorCodeCid,
					ConstructorParams: constructorParams,
				})
			if err != nil {
				return nil, err
			}
		}

		// Initialize miner actors
		for addr, val := range genCfg.miners {
			a := miner.NewActor()
			a.Balance = val.balance
			if err := st.SetActor(context.Background(), addr, a); err != nil {
				return nil, err
			}
		}
		for addr, nonce := range genCfg.nonces {
			a, err := st.GetActor(ctx, addr)
			if err != nil {
				return nil, err
			}
			a.CallSeqNum = types.Uint64(nonce)
			if err := st.SetActor(ctx, addr, a); err != nil {
				return nil, err
			}
		}
		// Now add any other actors configured.
		for addr, a := range genCfg.actors {
			if err := st.SetActor(ctx, addr, a); err != nil {
				return nil, err
			}
		}
		if err := actor.InitBuiltinActorCodeObjs(bs); err != nil {
			return nil, err
		}

		err := store.Flush()
		if err != nil {
			return nil, err
		}

		c, err := st.Flush(ctx)
		if err != nil {
			return nil, err
		}

		emptyAMTCid, err := amt.FromArray(ctx, cborutil.NewIpldStore(bs), []typegen.CBORMarshaler{})
		if err != nil {
			return nil, err
		}

		emptyBLSSignature := bls.Aggregate([]bls.Signature{})
		emptyMeta := types.TxMeta{SecpRoot: e.NewCid(emptyAMTCid), BLSRoot: e.NewCid(emptyAMTCid)}
		emptyMetaCid, err := cst.Put(ctx, emptyMeta)
		if err != nil {
			return nil, err
		}

		genesis := &block.Block{
			StateRoot:       e.NewCid(c),
			Messages:        e.NewCid(emptyMetaCid),
			MessageReceipts: e.NewCid(emptyAMTCid),
			BLSAggregateSig: emptyBLSSignature[:],
			Ticket:          block.Ticket{VRFProof: []byte{0xec}},
			Timestamp:       uint64(genCfg.genesisTimestamp.Unix()),
		}

		if _, err := cst.Put(ctx, genesis); err != nil {
			return nil, err
		}

		return genesis, nil
	}
}

// SetupDefaultActors inits the builtin actors that are required to run filecoin.
func SetupDefaultActors(ctx context.Context, vm GenesisVM, store *vm.Storage, st state.Tree, storeType types.ProofsMode, network string) error {
	createActor := func(addr address.Address, codeCid cid.Cid, state interface{}) *actor.Actor {

		a := actor.Actor{
			Code:       e.NewCid(codeCid),
			CallSeqNum: 0,
			Balance:    big.Zero(),
		}
		if state != nil {
			var err error
			headCid, err := store.Put(state)
			if err != nil {
				panic("failed to store state")
			}
			a.Head = e.NewCid(headCid)
		}
		if err := st.SetActor(context.Background(), addr, &a); err != nil {
			panic("failed to create actor during genesis block creation")
		}

		return &a
	}

	createActor(vmaddr.InitAddress, types.InitActorCodeCid, initactor.State{
		Network: network,
		NextID:  100,
	})

	createActor(vmaddr.StoragePowerAddress, types.PowerActorCodeCid, power.State{})

	createActor(vmaddr.StorageMarketAddress, types.StorageMarketActorCodeCid, storagemarket.State{
		TotalCommittedStorage: types.NewBytesAmount(0),
		ProofsMode:            storeType,
	})

	// sort addresses so genesis generation will be stable
	sortedAddresses := []string{}
	for addr := range defaultAccounts {
		sortedAddresses = append(sortedAddresses, string(addr.Bytes()))
	}
	sort.Strings(sortedAddresses)

	for _, addrBytes := range sortedAddresses {
		addr, err := address.NewFromBytes([]byte(addrBytes))
		if err != nil {
			return err
		}
		val := defaultAccounts[addr]
		if addr.Protocol() == address.ID {
			a, err := account.NewActor(val)
			if err != nil {
				return err
			}

			if err := st.SetActor(ctx, addr, a); err != nil {
				return err
			}
		} else {
			constructorParams, err := encoding.Encode(addr)
			if err != nil {
				return err
			}
			_, err = vm.ApplyGenesisMessage(vmaddr.LegacyNetworkAddress, vmaddr.InitAddress, initactor.ExecMethodID, val, initactor.ExecParams{
				ActorCodeCid:      types.AccountActorCodeCid,
				ConstructorParams: constructorParams,
			})
			if err != nil {
				return err
			}
		}

		if err != nil {
			return err
		}
	}

	return nil
}
