package main

import (
	"context"
	"log"

	"gx/ipfs/QmSkuaNgyGmV8c1L3cZNWcUxRJV6J3nsD96JVQPcWcwtyW/go-hamt-ipld"
	bserv "gx/ipfs/QmUSuYd5Q1N291DH679AVvHwGLwtS1V9VPDWvnUN9nGJPT/go-blockservice"
	"gx/ipfs/QmWdao8WJqYU65ZbYQyQWMFqku6QFxkPiv8HSUAkXdHZoe/go-ipfs-exchange-offline"
	"gx/ipfs/QmcD7SqfyQyA91TZUQ7VPRYbGarxmY7EsQewVYMuN5LNSv/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
)

func cmdFake(ctx context.Context, repodir string) error {
	r, err := repo.OpenFSRepo(repodir)
	if err != nil {
		return err
	}
	defer closeRepo(r)

	bs := blockstore.NewBlockstore(r.Datastore())
	cm, _ := getChainManager(r.Datastore(), bs)
	err = cm.Load()
	if err != nil {
		return err
	}

	aggregateState := func(ctx context.Context, ts core.TipSet) (state.Tree, error) {
		return cm.State(ctx, ts.ToSlice())
	}
	return fake(ctx, length, binom, cm.GetHeaviestTipSet, cm.ProcessNewBlock, aggregateState)
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
		log.Printf("Added chain of %d empty epochs.\n", length)
		return err
	}
	// The default block distribution just adds a linear chain of 1 block
	// per epoch.
	_, err := core.AddChain(ctx, processNewBlock, stateFromTS, ts.ToSlice(), length)
	if err != nil {
		return err
	}
	log.Printf("Added chain of %d empty blocks.\n", length)

	return err
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
