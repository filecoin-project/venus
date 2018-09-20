package main

import (
	"context"
	"log"

	"gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	bserv "gx/ipfs/QmTfTKeBhTLjSjxXQsjkF2b1DfZmYEMnknGE2y2gX57C6v/go-blockservice"
	"gx/ipfs/QmZxjqR9Qgompju73kakSoUj3rbVndAzky3oCDiBNCxPs1/go-ipfs-exchange-offline"
	"gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/api/impl"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func cmdFake(ctx context.Context, repodir string) error {
	r, err := repo.OpenFSRepo(repodir)
	if err != nil {
		return err
	}
	defer closeRepo(r)

	genCid, err := impl.LoadGenesis(r, th.GenesisFilePath())
	if err != nil {
		return err
	}

	tif := func(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*types.Block, error) {
		var blk types.Block

		if err := cst.Get(ctx, genCid, &blk); err != nil {
			return nil, err
		}

		return &blk, nil
	}

	bs := blockstore.NewBlockstore(r.Datastore())
	cm, _ := getChainManager(r.Datastore(), bs)

	err = cm.Genesis(ctx, tif)
	if err != nil {
		return err
	}

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
	return cm, cst
}
