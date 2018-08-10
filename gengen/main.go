package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/crypto"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"

	bserv "gx/ipfs/QmUSuYd5Q1N291DH679AVvHwGLwtS1V9VPDWvnUN9nGJPT/go-blockservice"
	"gx/ipfs/QmUe7hFx8ACivDWe1pF6X2ZTihGfeXppMc1aPjNBJ8cCHv/go-car"
	offline "gx/ipfs/QmWdao8WJqYU65ZbYQyQWMFqku6QFxkPiv8HSUAkXdHZoe/go-ipfs-exchange-offline"
	hamt "gx/ipfs/QmXJkSRxXHeAGmQJENct16anrKZHNECbmUoC7hMuCjLni6/go-hamt-ipld"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	blockstore "gx/ipfs/QmcD7SqfyQyA91TZUQ7VPRYbGarxmY7EsQewVYMuN5LNSv/go-ipfs-blockstore"
	dag "gx/ipfs/QmeCaeBmCCEJrZahwXY4G2G8zRaNBWskrfKWoQ6Xv6c1DR/go-merkledag"
	ds "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"
)

type Miner struct {
	// Owner is the name of the key that owns this miner
	Owner string

	// Power is the amount of power this miner should start off with
	// TODO: this will get more complicated when we actually have to
	// prove real files
	Power uint64
}

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
	rand.Read(buf)

	h, err := types.AddressHash(buf)
	if err != nil {
		panic(err)
	}
	return types.NewAddress(types.Mainnet, h)
}

// GenGen takes the genesis configuration and creates a genesis block that
// matches the description. It writes all chunks to the dagservice, and returns
// the final genesis block.
func GenGen(ctx context.Context, cfg *GenesisCfg, cst *hamt.CborIpldStore) (*cid.Cid, error) {
	keys := make(map[string]*types.KeyInfo)
	for _, k := range cfg.Keys {
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

	st := state.NewEmptyStateTree(cst)

	for k, v := range cfg.PreAlloc {
		ki, ok := keys[k]
		if !ok {
			return nil, fmt.Errorf("no such key: %q", k)
		}

		addr, err := ki.Address()
		if err != nil {
			return nil, err
		}

		valint, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, err
		}

		act := &types.Actor{
			Balance: types.NewAttoFILFromFIL(valint),
		}

		if err := st.SetActor(ctx, addr, act); err != nil {
			return nil, err
		}
	}

	smaStorage := &storagemarket.Storage{
		Miners: make(types.AddrSet),
		Orderbook: &storagemarket.Orderbook{
			Asks: make(storagemarket.AskSet),
			Bids: make(storagemarket.BidSet),
		},
		Filemap: &storagemarket.Filemap{
			Files: make(map[string][]uint64),
		},
	}

	for _, m := range cfg.Miners {
		addr, err := keys[m.Owner].Address()
		if err != nil {
			return nil, err
		}

		mst := &miner.Storage{
			Owner:         addr,
			PeerID:        "",
			PublicKey:     nil,
			PledgeBytes:   types.NewBytesAmount(10000000000),
			Collateral:    types.NewAttoFILFromFIL(100000),
			LockedStorage: types.NewBytesAmount(0),
			Power:         types.NewBytesAmount(m.Power),
		}

		storageBytes, err := actor.MarshalStorage(mst)
		if err != nil {
			return nil, err
		}

		act := types.NewActorWithMemory(types.MinerActorCodeCid, nil, storageBytes)

		maddr := randAddress()

		if err := st.SetActor(ctx, maddr, act); err != nil {
			return nil, err
		}
		smaStorage.Miners[maddr] = struct{}{}
	}

	storageBytes, err := actor.MarshalStorage(smaStorage)
	if err != nil {
		return nil, err
	}
	sma := types.NewActorWithMemory(types.StorageMarketActorCodeCid, nil, storageBytes)

	if err := st.SetActor(ctx, address.StorageMarketAddress, sma); err != nil {
		return nil, err
	}

	stateRoot, err := st.Flush(ctx)
	if err != nil {
		return nil, err
	}

	geneblk := &types.Block{
		StateRoot: stateRoot,
	}

	return cst.Put(ctx, geneblk)
}

func main() {
	var cfg GenesisCfg
	if err := json.NewDecoder(os.Stdin).Decode(&cfg); err != nil {
		panic(err)
	}

	mds := ds.NewMapDatastore()
	bstore := blockstore.NewBlockstore(mds)
	offl := offline.Exchange(bstore)
	blkserv := bserv.New(bstore, offl)
	cst := &hamt.CborIpldStore{blkserv}
	dserv := dag.NewDAGService(blkserv)

	ctx := context.Background()

	c, err := GenGen(ctx, &cfg, cst)
	if err != nil {
		panic(err)
	}

	if err := car.WriteCar(ctx, dserv, c, os.Stdout); err != nil {
		panic(err)
	}
}
