package gengen

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
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

	hamt "gx/ipfs/QmSkuaNgyGmV8c1L3cZNWcUxRJV6J3nsD96JVQPcWcwtyW/go-hamt-ipld"
	"gx/ipfs/QmU9oYpqJsNWwAAJju8CzE7mv4NHAJUDWhoKHqgnhMCBy5/go-car"
	bserv "gx/ipfs/QmUSuYd5Q1N291DH679AVvHwGLwtS1V9VPDWvnUN9nGJPT/go-blockservice"
	offline "gx/ipfs/QmWdao8WJqYU65ZbYQyQWMFqku6QFxkPiv8HSUAkXdHZoe/go-ipfs-exchange-offline"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	blockstore "gx/ipfs/QmcD7SqfyQyA91TZUQ7VPRYbGarxmY7EsQewVYMuN5LNSv/go-ipfs-blockstore"
	peer "gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"
	dag "gx/ipfs/QmeCaeBmCCEJrZahwXY4G2G8zRaNBWskrfKWoQ6Xv6c1DR/go-merkledag"
	ds "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"
)

// Miner is
type Miner struct {
	// Owner is the name of the key that owns this miner
	// It must be a name of a key from the configs 'Keys' list
	Owner string

	// PeerID is the peer ID to set as the miners owner
	PeerID string

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

	h := types.AddressHash(buf)
	return types.NewAddress(types.Mainnet, h)
}

// RenderedGenInfo contains information about a genesis block creation
type RenderedGenInfo struct {
	// Keys is the set of keys generated
	Keys map[string]*types.KeyInfo

	// Miners is the list of addresses of miners created
	Miners []RenderedMinerInfo

	// GenesisCid is the cid of the created genesis block
	GenesisCid *cid.Cid
}

// RenderedMinerInfo contains info about a created miner
type RenderedMinerInfo struct {
	// Owner is the key name of the owner of this miner
	Owner string

	// Address is the address generated on-chain for the miner
	Address types.Address

	// Power is the amount of storage power this miner was created with
	Power uint64
}

// GenGen takes the genesis configuration and creates a genesis block that
// matches the description. It writes all chunks to the dagservice, and returns
// the final genesis block.
func GenGen(ctx context.Context, cfg *GenesisCfg, cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*RenderedGenInfo, error) {
	keys, err := genKeys(cfg.Keys)
	if err != nil {
		return nil, err
	}

	st := state.NewEmptyStateTree(cst)
	storageMap := vm.NewStorageMap(bs)

	if err := core.SetupDefaultActors(ctx, st, storageMap); err != nil {
		return nil, err
	}

	if err := setupPrealloc(st, keys, cfg.PreAlloc); err != nil {
		return nil, err
	}

	miners, err := setupMiners(st, cst, keys, cfg.Miners)
	if err != nil {
		return nil, err
	}

	if err := cst.Blocks.AddBlock(types.StorageMarketActorCodeObj); err != nil {
		return nil, err
	}
	if err := cst.Blocks.AddBlock(types.MinerActorCodeObj); err != nil {
		return nil, err
	}
	if err := cst.Blocks.AddBlock(types.AccountActorCodeObj); err != nil {
		return nil, err
	}
	if err := cst.Blocks.AddBlock(types.PaymentBrokerActorCodeObj); err != nil {
		return nil, err
	}

	stateRoot, err := st.Flush(ctx)
	if err != nil {
		return nil, err
	}

	err = storageMap.Flush()
	if err != nil {
		return nil, err
	}

	geneblk := &types.Block{
		StateRoot: stateRoot,
	}

	c, err := cst.Put(ctx, geneblk)
	if err != nil {
		return nil, err
	}

	return &RenderedGenInfo{
		Keys:       keys,
		GenesisCid: c,
		Miners:     miners,
	}, nil
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

func setupMiners(st state.Tree, cst *hamt.CborIpldStore, keys map[string]*types.KeyInfo, miners []Miner) ([]RenderedMinerInfo, error) {
	smaStorage := &storagemarket.State{
		Miners: make(types.AddrSet),
		Orderbook: &storagemarket.Orderbook{
			StorageAsks: make(storagemarket.AskSet),
			Bids:        make(storagemarket.BidSet),
		},
	}

	var minfos []RenderedMinerInfo
	powerSum := types.NewBytesAmount(0)
	for _, m := range miners {
		addr, err := keys[m.Owner].Address()
		if err != nil {
			return nil, err
		}

		var pid peer.ID
		if m.PeerID != "" {
			p, err := peer.IDB58Decode(m.PeerID)
			if err != nil {
				return nil, err
			}
			pid = p
		}

		mst := &miner.State{
			Owner:         addr,
			PeerID:        pid,
			PublicKey:     nil,
			PledgeBytes:   types.NewBytesAmount(10000000000),
			Collateral:    types.NewAttoFILFromFIL(100000),
			LockedStorage: types.NewBytesAmount(0),
			Power:         types.NewBytesAmount(m.Power),
		}
		powerSum = powerSum.Add(mst.Power)

		root, err := cst.Put(context.Background(), mst)
		if err != nil {
			return nil, err
		}

		act := miner.NewActor()
		act.Head = root

		maddr := randAddress()

		if err := st.SetActor(context.Background(), maddr, act); err != nil {
			return nil, err
		}
		smaStorage.Miners[maddr] = struct{}{}
		minfos = append(minfos, RenderedMinerInfo{
			Address: maddr,
			Owner:   m.Owner,
			Power:   m.Power,
		})
	}

	smaStorage.TotalCommittedStorage = powerSum

	root, err := cst.Put(context.Background(), smaStorage)
	if err != nil {
		return nil, err
	}
	sma, err := storagemarket.NewActor()
	if err != nil {
		return err
	}
	sma.Head = root

	if err := st.SetActor(context.Background(), address.StorageMarketAddress, sma); err != nil {
		return nil, err
	}

	return minfos, nil
}

// GenGenesisCar generates a car for the given genesis configuration
func GenGenesisCar(cfg *GenesisCfg, out io.Writer) (*RenderedGenInfo, error) {
	// TODO: these six lines are ugly. We can do better...
	mds := ds.NewMapDatastore()
	bstore := blockstore.NewBlockstore(mds)
	offl := offline.Exchange(bstore)
	blkserv := bserv.New(bstore, offl)
	cst := &hamt.CborIpldStore{Blocks: blkserv}
	dserv := dag.NewDAGService(blkserv)

	ctx := context.Background()

	info, err := GenGen(ctx, cfg, cst, bstore)
	if err != nil {
		return nil, err
	}

	return info, car.WriteCar(ctx, dserv, []*cid.Cid{info.GenesisCid}, out)
}
