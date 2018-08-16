package gengen

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/crypto"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"

	"gx/ipfs/QmPtncU95mqpLvp5G8xDytqyBfU11SWJwSFFunPhMUTWAw/go-car"
	bserv "gx/ipfs/QmUSuYd5Q1N291DH679AVvHwGLwtS1V9VPDWvnUN9nGJPT/go-blockservice"
	offline "gx/ipfs/QmWdao8WJqYU65ZbYQyQWMFqku6QFxkPiv8HSUAkXdHZoe/go-ipfs-exchange-offline"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	hamt "gx/ipfs/QmbwwhSsEcSPP4XfGumu6GMcuCLnCLVQAnp3mDxKuYNXJo/go-hamt-ipld"
	blockstore "gx/ipfs/QmcD7SqfyQyA91TZUQ7VPRYbGarxmY7EsQewVYMuN5LNSv/go-ipfs-blockstore"
	dag "gx/ipfs/QmeCaeBmCCEJrZahwXY4G2G8zRaNBWskrfKWoQ6Xv6c1DR/go-merkledag"
	ds "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"
)

// Miner is
type Miner struct {
	// Owner is the name of the key that owns this miner
	// It must be a name of a key from the configs 'Keys' list
	Owner string

	// Power is the amount of power this miner should start off with
	// TODO: this will get more complicated when we actually have to
	// prove real files
	Power uint64
}

// GenesisCfg is
type GenesisCfg struct {
	// Keys is an array of names of keys. A random key will be generated
	// for each name here.
	Keys []string

	// PreAlloc is a mapping from key names to string values of whole filecoin
	// that will be preallocated to each account
	PreAlloc map[string]string

	// Miners is a list of miners that should be set up at the start of the network
	Miners []Miner
}

func randAddress() types.Address {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)

	h, err := types.AddressHash(buf)
	if err != nil {
		panic(err)
	}
	return types.NewAddress(types.Mainnet, h)
}

// GenGen takes the genesis configuration and creates a genesis block that
// matches the description. It writes all chunks to the dagservice, and returns
// the final genesis block.
func GenGen(ctx context.Context, cfg *GenesisCfg, cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*cid.Cid, map[string]*types.KeyInfo, error) {
	keys, err := genKeys(cfg.Keys)
	if err != nil {
		return nil, nil, err
	}

	st := state.NewEmptyStateTree(cst)
	storageMap := vm.NewStorageMap(bs)

	if err := core.SetupDefaultActors(ctx, st, storageMap); err != nil {
		return nil, nil, err
	}

	if err := setupPrealloc(st, keys, cfg.PreAlloc); err != nil {
		return nil, nil, err
	}

	if err := setupMiners(st, cst, keys, cfg.Miners); err != nil {
		return nil, nil, err
	}

	if err := cst.Blocks.AddBlock(types.StorageMarketActorCodeObj); err != nil {
		return nil, nil, err
	}
	if err := cst.Blocks.AddBlock(types.MinerActorCodeObj); err != nil {
		return nil, nil, err
	}
	if err := cst.Blocks.AddBlock(types.AccountActorCodeObj); err != nil {
		return nil, nil, err
	}
	if err := cst.Blocks.AddBlock(types.PaymentBrokerActorCodeObj); err != nil {
		return nil, nil, err
	}

	stateRoot, err := st.Flush(ctx)
	if err != nil {
		return nil, nil, err
	}

	err = storageMap.Flush()
	if err != nil {
		return nil, nil, err
	}

	geneblk := &types.Block{
		StateRoot: stateRoot,
	}

	c, err := cst.Put(ctx, geneblk)
	if err != nil {
		return nil, nil, err
	}

	return c, keys, nil
}

func genKeys(cfgkeys []string) (map[string]*types.KeyInfo, error) {
	keys := make(map[string]*types.KeyInfo)
	for _, k := range cfgkeys {
		if _, ok := keys[k]; ok {
			return nil, fmt.Errorf("duplicate key with name: %q", k)
		}
		sk, err := crypto.GenerateKey() // TODO: GenerateKey should return a KeyInfo
		if err != nil {
			return nil, err
		}

		ki := &types.KeyInfo{
			PrivateKey: crypto.ECDSAToBytes(sk),
			Curve:      types.SECP256K1,
		}

		keys[k] = ki
	}

	return keys, nil
}

func setupPrealloc(st state.Tree, keys map[string]*types.KeyInfo, prealloc map[string]string) error {

	for k, v := range prealloc {
		ki, ok := keys[k]
		if !ok {
			return fmt.Errorf("no such key: %q", k)
		}

		addr, err := ki.Address()
		if err != nil {
			return err
		}

		valint, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return err
		}

		act, err := account.NewActor(types.NewAttoFILFromFIL(valint))
		if err != nil {
			return err
		}
		if err := st.SetActor(context.Background(), addr, act); err != nil {
			return err
		}
	}

	netact, err := account.NewActor(types.NewAttoFILFromFIL(10000000000))
	if err != nil {
		return err
	}

	return st.SetActor(context.Background(), address.NetworkAddress, netact)
}

func setupMiners(st state.Tree, cst *hamt.CborIpldStore, keys map[string]*types.KeyInfo, miners []Miner) error {
	smaStorage := &storagemarket.State{
		Miners: make(types.AddrSet),
		Orderbook: &storagemarket.Orderbook{
			Asks: make(storagemarket.AskSet),
			Bids: make(storagemarket.BidSet),
		},
		Filemap: &storagemarket.Filemap{
			Files: make(map[string][]uint64),
		},
	}

	powerSum := types.NewBytesAmount(0)
	for _, m := range miners {
		addr, err := keys[m.Owner].Address()
		if err != nil {
			return err
		}

		mst := &miner.State{
			Owner:         addr,
			PeerID:        "",
			PublicKey:     nil,
			PledgeBytes:   types.NewBytesAmount(10000000000),
			Collateral:    types.NewAttoFILFromFIL(100000),
			LockedStorage: types.NewBytesAmount(0),
			Power:         types.NewBytesAmount(m.Power),
		}
		powerSum = powerSum.Add(mst.Power)

		root, err := cst.Put(context.Background(), mst)
		if err != nil {
			return err
		}

		act := types.NewActor(types.MinerActorCodeCid, nil)
		act.Head = root

		maddr := randAddress()

		if err := st.SetActor(context.Background(), maddr, act); err != nil {
			return err
		}
		smaStorage.Miners[maddr] = struct{}{}
		fmt.Fprintf(os.Stderr, "created miner %s, owned by %s, power = %d\n", maddr, m.Owner, m.Power)
	}

	smaStorage.TotalCommittedStorage = powerSum

	root, err := cst.Put(context.Background(), smaStorage)
	if err != nil {
		return err
	}
	sma := types.NewActor(types.StorageMarketActorCodeCid, nil)
	sma.Head = root
	fmt.Fprintln(os.Stderr, "storage market actor head: ", root)

	return st.SetActor(context.Background(), address.StorageMarketAddress, sma)
}

// GenGenesisCar generates a car for the given genesis configuration
func GenGenesisCar(cfg *GenesisCfg, out io.Writer) (map[string]*types.KeyInfo, error) {
	// TODO: these six lines are ugly. We can do better...
	mds := ds.NewMapDatastore()
	bstore := blockstore.NewBlockstore(mds)
	offl := offline.Exchange(bstore)
	blkserv := bserv.New(bstore, offl)
	cst := &hamt.CborIpldStore{Blocks: blkserv}
	dserv := dag.NewDAGService(blkserv)

	ctx := context.Background()

	c, keys, err := GenGen(ctx, cfg, cst, bstore)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error from gengen?")
		return nil, err
	}

	return keys, car.WriteCar(ctx, dserv, []*cid.Cid{c}, out)
}
