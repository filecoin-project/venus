package main

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/crypto"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"

	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-car"
	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	hamt "github.com/ipfs/go-hamt-ipld"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	format "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"
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

// GenGen takes the genesis configuration and creates a genesis block that
// matches the description. It writes all chunks to the dagservice, and returns
// the final genesis block.
func GenGen(cfg *GenesisCfg, cst *hamt.CborIpldStore) (*cid.Cid, error) {
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
			PrivateKey: sk,
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

	c, err := GenGen(&cfg, cst)
	if err != nil {
		panic(err)
	}

	if err := car.WriteCar(ctx, dserv, c, os.Stdout); err != nil {
		panic(err)
	}
}
