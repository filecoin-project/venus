package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"

	"gx/ipfs/QmSkuaNgyGmV8c1L3cZNWcUxRJV6J3nsD96JVQPcWcwtyW/go-hamt-ipld"
	bserv "gx/ipfs/QmUSuYd5Q1N291DH679AVvHwGLwtS1V9VPDWvnUN9nGJPT/go-blockservice"
	"gx/ipfs/QmWdao8WJqYU65ZbYQyQWMFqku6QFxkPiv8HSUAkXdHZoe/go-ipfs-exchange-offline"
	"gx/ipfs/QmcD7SqfyQyA91TZUQ7VPRYbGarxmY7EsQewVYMuN5LNSv/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

var length int
var binom bool
var repodir string

func init() {
	flag.IntVar(&length, "length", 5, "length of fake chain to create")

	// Default repodir is different than Filecoin to avoid accidental clobbering of real data.
	flag.StringVar(&repodir, "repodir", "~/.fakecoin", "repo directory to use")

	flag.BoolVar(&binom, "binom", false, "generate multiblock tipsets where the number of blocks per epoch is drawn from a a realistic distribution")
}

func main() {
	ctx := context.Background()

	var cmd string

	if len(os.Args) > 1 {
		cmd = os.Args[1]
		if len(os.Args) > 2 {
			// Remove the cmd argument if there are options, to satisfy flag.Parse() while still allowing a command-first syntax.
			os.Args = append(os.Args[1:], os.Args[0])
		}
	}
	flag.Parse()
	switch cmd {
	default:
		flag.Usage()
	case "fake":
		r, err := repo.OpenFSRepo(repodir)
		if err != nil {
			log.Fatal(err)
		}
		defer closeRepo(r)

		bs := blockstore.NewBlockstore(r.Datastore())
		cm, _ := getChainManager(r.Datastore(), bs)
		err = cm.Load()
		if err != nil {
			log.Fatal(err)
		}

		aggregateState := func(ctx context.Context, ts core.TipSet) (state.Tree, error) {
			return cm.State(ctx, ts.ToSlice())
		}
		err = fake(ctx, length, binom, cm.GetHeaviestTipSet, cm.ProcessNewBlock, aggregateState)
		if err != nil {
			log.Fatal(err)
		}
	// TODO: Make usage message reflect the command argument.

	case "actors":
		r, err := repo.OpenFSRepo(repodir)
		if err != nil {
			log.Fatal(err)
		}
		defer closeRepo(r)
		bs := blockstore.NewBlockstore(r.Datastore())

		_, cst, cm, bts, err := getStateTree(ctx, r.Datastore(), bs)
		if err != nil {
			log.Fatal(err)
		}
		err = fakeActors(ctx, cst, cm, bs, bts)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func closeRepo(r *repo.FSRepo) {
	err := r.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func getChainManager(d repo.Datastore, bs blockstore.Blockstore) (*core.ChainManager, *hamt.CborIpldStore) {
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	cm := core.NewChainManager(d, bs, cst)
	// allow fakecoin to mine without having a correct storage market / state tree
	cm.PwrTableView = &core.TestView{}
	return cm, cst
}

func getBlockGenerator(msgPool *core.MessagePool, cm *core.ChainManager, cst *hamt.CborIpldStore, bs blockstore.Blockstore) mining.BlockGenerator {
	return mining.NewBlockGenerator(msgPool, func(ctx context.Context, ts core.TipSet) (state.Tree, error) {
		return cm.State(ctx, ts.ToSlice())
	}, cm.Weight, core.ApplyMessages, cm.PwrTableView, bs, cst)
}

func getStateTree(ctx context.Context, d repo.Datastore, bs blockstore.Blockstore) (state.Tree, *hamt.CborIpldStore, *core.ChainManager, core.TipSet, error) {
	cm, cst := getChainManager(d, bs)
	err := cm.Load()
	if err != nil {
		log.Fatal(err)
	}

	bts := cm.GetHeaviestTipSet()
	st, err := cm.State(ctx, bts.ToSlice())
	return st, cst, cm, bts, err
}

func fake(ctx context.Context, length int, binom bool, getHeaviestTipSet core.HeaviestTipSetGetter, processNewBlock core.NewBlockProcessor, stateFromTS core.AggregateStateTreeComputer) error {
	ts := getHeaviestTipSet()
	// If a binomial distribution is specified we generate a binomially
	// distributed number of blocks per epoch
	if binom {
		_, err := core.AddChainBinomBlocksPerEpoch(ctx, processNewBlock, stateFromTS, ts, 100, length)
		if err != nil {
			return err
		}
		fmt.Printf("Added chain of %d empty epochs.\n", length)
		return err
	}
	// The default block distribution just adds a linear chain of 1 block
	// per epoch.
	_, err := core.AddChain(ctx, processNewBlock, stateFromTS, ts.ToSlice(), length)
	if err != nil {
		return err
	}
	fmt.Printf("Added chain of %d empty blocks.\n", length)

	return err
}

// fakeActors adds a block ensuring that the StateTree contains at least one of each extant Actor type, along with
// well-formed data in its memory. For now, this exists primarily to exercise the Filecoin Explorer, though it may
// be used for testing in the future.
func fakeActors(ctx context.Context, cst *hamt.CborIpldStore, cm *core.ChainManager, bs blockstore.Blockstore, bts core.TipSet) (err error) {
	msgPool := core.NewMessagePool()

	//// Have the storage market actor create a new miner
	params, err := abi.ToEncodedValues(types.NewBytesAmount(100000), []byte{}, core.RequireRandomPeerID())
	if err != nil {
		return err
	}

	// TODO address support for signed messages
	newMinerMessage := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(400), "createMiner", params)
	newSingedMinerMessage, err := types.NewSignedMessage(*newMinerMessage, nil)
	if err != nil {
		return err
	}
	_, err = msgPool.Add(newSingedMinerMessage)
	if err != nil {
		return err
	}

	blk, err := mineBlock(ctx, msgPool, cst, cm, bs, bts.ToSlice())
	if err != nil {
		return err
	}
	msgPool = core.NewMessagePool()

	cid, err := newMinerMessage.Cid()
	if err != nil {
		return err
	}

	var createMinerReceipt *types.MessageReceipt
	err = cm.WaitForMessage(ctx, cid, func(b *types.Block, msg *types.SignedMessage, rcp *types.MessageReceipt) error {
		createMinerReceipt = rcp
		return nil
	})
	if err != nil {
		return err
	}

	minerAddress, err := types.NewAddressFromBytes(createMinerReceipt.Return[0])
	if err != nil {
		return err
	}

	// Add a new ask to the storage market
	params, err = abi.ToEncodedValues(types.NewAttoFILFromFIL(10), types.NewBytesAmount(1000))
	if err != nil {
		return err
	}
	// TODO address support for signed messages
	askMsg := types.NewMessage(address.TestAddress, minerAddress, 1, types.NewAttoFILFromFIL(100), "addAsk", params)
	askSignedMessage, err := types.NewSignedMessage(*askMsg, nil)
	if err != nil {
		return err
	}
	_, err = msgPool.Add(askSignedMessage)
	if err != nil {
		return err
	}

	// Add a new bid to the storage market
	params, err = abi.ToEncodedValues(types.NewAttoFILFromFIL(9), types.NewBytesAmount(10))
	if err != nil {
		return err
	}
	// TODO address support for signed messages
	bidMsg := types.NewMessage(address.TestAddress2, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(90), "addBid", params)
	bidSignedMessage, err := types.NewSignedMessage(*bidMsg, nil)
	if err != nil {
		return err
	}
	_, err = msgPool.Add(bidSignedMessage)
	if err != nil {
		return err
	}

	// mine again
	blk, err = mineBlock(ctx, msgPool, cst, cm, bs, []*types.Block{blk})
	if err != nil {
		return err
	}
	msgPool = core.NewMessagePool()

	// Create deal
	params, err = abi.ToEncodedValues(big.NewInt(0), big.NewInt(0), address.TestAddress2, types.NewCidForTestGetter()().Bytes())
	if err != nil {
		return err
	}
	// TODO address support for signed messages
	newDealMessage := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 2, types.NewAttoFILFromFIL(400), "addDeal", params)
	newDealSignedMessage, err := types.NewSignedMessage(*newDealMessage, nil)
	if err != nil {
		return err
	}
	_, err = msgPool.Add(newDealSignedMessage)
	if err != nil {
		return err
	}

	_, err = mineBlock(ctx, msgPool, cst, cm, bs, []*types.Block{blk})
	return err
}

func mineBlock(ctx context.Context, mp *core.MessagePool, cst *hamt.CborIpldStore, cm *core.ChainManager, bs blockstore.Blockstore, blks []*types.Block) (*types.Block, error) {
	bg := getBlockGenerator(mp, cm, cst, bs)
	ma := types.MakeTestAddress("miningaddress")

	const nullBlockCount = 0
	ts, err := core.NewTipSet(blks...)
	if err != nil {
		return nil, err
	}
	blk, err := bg.Generate(ctx, ts, nil, nullBlockCount, ma)
	if err != nil {
		return nil, err
	}

	_, err = cm.ProcessNewBlock(ctx, blk)
	if err != nil {
		return nil, err
	}

	return blk, nil
}
