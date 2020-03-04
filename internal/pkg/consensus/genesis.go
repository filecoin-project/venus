package consensus

import (
	"context"
	"runtime/debug"
	"sort"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-amt-ipld/v2"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"
	typegen "github.com/whyrusleeping/cbor-gen"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vmsupport"
)

// GenesisInitFunc is the signature for function that is used to create a genesis block.
type GenesisInitFunc func(cst cbor.IpldStore, bs blockstore.Blockstore) (*block.Block, error)

// GenesisTicket is the ticket to place in the genesis block header (which can't be derived from a prior ticket),
// used in the evaluation of the messages in the genesis block,
// and *also* the ticket value used when computing the genesis state (the parent state of the genesis block).
var GenesisTicket = block.Ticket{VRFProof: []byte{0xec}}

var (
	defaultAccounts map[address.Address]abi.TokenAmount
)

func init() {
	defaultAccounts = map[address.Address]abi.TokenAmount{
		builtin.RewardActorAddr:     abi.NewTokenAmount(10000000000),
		builtin.BurntFundsActorAddr: abi.NewTokenAmount(0),
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
func MinerActor(store adt.Store, addr address.Address, owner address.Address, pid peer.ID, coll abi.TokenAmount, sectorSize abi.SectorSize) GenOption {
	return func(gc *Config) error {
		emptyMap, err := adt.MakeEmptyMap(store)
		if err != nil {
			return err
		}

		emptyArray, err := adt.MakeEmptyArray(store)
		if err != nil {
			return err
		}

		st := miner.ConstructState(emptyArray.Root(), emptyMap.Root(), owner, owner, pid, sectorSize)
		if err != nil {
			return err
		}

		gc.miners[addr] = &minerActorConfig{
			state:   st,
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
	ApplyGenesisMessage(from address.Address, to address.Address, method abi.MethodNum, value abi.TokenAmount, params interface{}, rnd crypto.RandomnessSource) (interface{}, error)
	ContextStore() adt.Store
}

// MakeGenesisFunc returns a genesis function configured by a set of options.
func MakeGenesisFunc(opts ...GenOption) GenesisInitFunc {
	return func(cst cbor.IpldStore, bs blockstore.Blockstore) (*block.Block, error) {
		ctx := context.Background()
		st := state.NewState(cst)
		store := vm.NewStorage(bs)
		vm := vm.NewVM(st, &store, vmsupport.NewSyscalls()).(GenesisVM)
		rnd := crypto.ChainRandomnessSource{Sampler: &crypto.GenesisSampler{VRFProof: GenesisTicket.VRFProof}}

		genCfg := NewEmptyConfig()
		for _, opt := range opts {
			if err := opt(genCfg); err != nil {
				return nil, err
			}
		}

		if err := SetupDefaultActors(ctx, vm, &store, st, &rnd, genCfg.network); err != nil {
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

			_, err = vm.ApplyGenesisMessage(builtin.RewardActorAddr, addr, builtin.MethodSend, val, nil, &rnd)
			if err != nil {
				return nil, err
			}
		}

		// Initialize miner actors
		for addr, val := range genCfg.miners {
			a := actor.NewActor(builtin.StorageMinerActorCodeID, val.balance)
			if err := st.SetActor(context.Background(), addr, a); err != nil {
				return nil, err
			}
		}
		for addr, nonce := range genCfg.nonces {
			a, found, err := st.GetActor(ctx, addr)
			if err != nil {
				return nil, err
			}
			if !found {
				panic("unreachable: actor not found")
			}
			a.CallSeqNum = nonce
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
		err := store.Flush()
		if err != nil {
			return nil, err
		}

		root, err := st.Commit(ctx)
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
			StateRoot:       e.NewCid(root),
			Messages:        e.NewCid(emptyMetaCid),
			MessageReceipts: e.NewCid(emptyAMTCid),
			BLSAggregateSig: crypto.Signature{
				Type: crypto.SigTypeBLS,
				Data: emptyBLSSignature[:],
			},
			Ticket:    GenesisTicket,
			Timestamp: uint64(genCfg.genesisTimestamp.Unix()),
			BlockSig:  crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: []byte{}},
		}

		if _, err := cst.Put(ctx, genesis); err != nil {
			return nil, err
		}

		return genesis, nil
	}
}

// SetupDefaultActors inits the builtin actors that are required to run filecoin.
func SetupDefaultActors(ctx context.Context, vm GenesisVM, store *vm.Storage, st *state.State, rnd crypto.RandomnessSource, network string) error {
	createActor := func(addr address.Address, codeCid cid.Cid, balance abi.TokenAmount, stateFn func() (interface{}, error)) *actor.Actor {

		a := actor.Actor{
			Code:       e.NewCid(codeCid),
			CallSeqNum: 0,
			Balance:    balance,
		}
		if stateFn != nil {
			state, err := stateFn()
			if err != nil {
				debug.PrintStack()
				panic("failed to create state")
			}
			headCid, _, err := store.Put(ctx, state)
			if err != nil {
				debug.PrintStack()
				panic("failed to store state")
			}
			a.Head = e.NewCid(headCid)
		}
		if err := st.SetActor(context.Background(), addr, &a); err != nil {
			panic("failed to create actor during genesis block creation")
		}

		return &a
	}
	emptyArray, err := adt.MakeEmptyArray(vm.ContextStore())
	if err != nil {
		return err
	}
	emptyMap, err := adt.MakeEmptyMap(vm.ContextStore())
	if err != nil {
		return err
	}
	emptyMSet, err := market.MakeEmptySetMultimap(vm.ContextStore())
	if err != nil {
		return err
	}

	createActor(builtin.InitActorAddr, builtin.InitActorCodeID, big.Zero(), func() (interface{}, error) {
		return init_.ConstructState(emptyMap.Root(), network), nil
	})

	createActor(builtin.StoragePowerActorAddr, builtin.StoragePowerActorCodeID, big.Zero(), func() (interface{}, error) {
		return power.ConstructState(emptyMap.Root()), nil
	})

	createActor(builtin.StorageMarketActorAddr, builtin.StorageMarketActorCodeID, big.Zero(), func() (interface{}, error) {
		return market.ConstructState(emptyArray.Root(), emptyMap.Root(), emptyMSet.Root()), nil
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
			a := actor.NewActor(builtin.AccountActorCodeID, val)

			if err := st.SetActor(ctx, addr, a); err != nil {
				return err
			}
		} else {
			_, err = vm.ApplyGenesisMessage(builtin.RewardActorAddr, addr, builtin.MethodSend, val, nil, rnd)
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
