package main

import (
	"context"
	"log"

	"gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	bserv "gx/ipfs/QmTfTKeBhTLjSjxXQsjkF2b1DfZmYEMnknGE2y2gX57C6v/go-blockservice"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZxjqR9Qgompju73kakSoUj3rbVndAzky3oCDiBNCxPs1/go-ipfs-exchange-offline"
	"gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/api/impl"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
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
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	genesis, err := tif(cst, bs)
	if err != nil {
		return err
	}
	genTS, err := consensus.NewTipSet(genesis)
	if err != nil {
		return err
	}
	chainStore := chain.NewDefaultStore(r.Datastore(), cst, genCid)
	defer chainStore.Stop()
	err = chainStore.PutTipSetAndState(ctx, &chain.TipSetAndState{
		TipSet:          genTS,
		TipSetStateRoot: genesis.StateRoot,
	})
	if err != nil {
		return err
	}
	err = chainStore.SetHead(ctx, genTS)
	if err != nil {
		return err
	}

	err = chainStore.Load(ctx)
	if err != nil {
		return err
	}

	return fake(ctx, length, binom, chainStore)
}

func fake(ctx context.Context, length int, binom bool, chainStore chain.Store) error {
	ts := chainStore.Head()
	if ts == nil {
		return errors.New("head of chain unset")
	}
	// If a binomial distribution is specified we generate a binomially
	// distributed number of blocks per epoch
	if binom {
		defaultNumMiners := 100 // TODO make this configurable in binom flag
		_, err := chain.AddChainBinomBlocksPerEpoch(ctx, chainStore, ts.ToSlice(), defaultNumMiners, length)
		if err != nil {
			return err
		}
		log.Printf("Added chain of %d empty epochs.\n", length)
		return err
	}
	// The default block distribution just adds a linear chain of 1 block
	// per epoch.
	_, err := chain.AddChain(ctx, chainStore, ts.ToSlice(), length)
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
