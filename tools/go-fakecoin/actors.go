package main

import (
	"context"
	"log"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit/files"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/api/impl"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/repo"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func cmdFakeActors(ctx context.Context, repodir string) error {
	log.Printf("faking actors in %s", repodir)

	log.Println("\trunning init")

	fc := impl.New(nil)
	err := fc.Daemon().Init(
		ctx,
		api.RepoDir(repodir),
		api.GenesisFile(th.GenesisFilePath()),
	)
	if err != nil {
		return errors.Wrap(err, "failed to init daemon")
	}

	rep, err := repo.OpenFSRepo(repodir)
	if err != nil {
		return errors.Wrap(err, "failed to open fsrepo")
	}

	opts, err := node.OptionsFromRepo(rep)
	if err != nil {
		return errors.Wrap(err, "failed to derive options from repo")
	}

	opts = append(opts, node.OfflineMode(true))
	opts = append(opts, node.MockMineMode(true))
	opts = append(opts, node.BlockTime(10*time.Millisecond))

	fcn, err := node.New(ctx, opts...)
	if err != nil {
		return errors.Wrap(err, "failed to create node")
	}

	fc = impl.New(fcn)

	log.Println("\tstarting daemon")
	if err := fc.Daemon().Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start daemon")
	}

	log.Println("\tloading test addresses")

	fa, err := readFiles(th.KeyFilePaths())
	if err != nil {
		return errors.Wrap(err, "failed to read keyfiles")
	}

	if _, err := fc.Address().Import(ctx, fa); err != nil {
		return errors.Wrap(err, "failed to import test address")
	}

	if err := fakeActors(ctx, fc); err != nil {
		return err
	}

	log.Println("\tstopping daemon")
	if err := fc.Daemon().Stop(ctx); err != nil {
		return errors.Wrap(err, "failed to stop daemon")
	}
	return nil
}

// fakeActors adds a block ensuring that the StateTree contains at least one of each extant Actor type, along with
// well-formed data in its memory. For now, this exists primarily to exercise the Filecoin Explorer, though it may
// be used for testing in the future.
func fakeActors(ctx context.Context, fc api.API) error {
	clientAddr, err := address.NewFromString(th.TestAddress1)
	if err != nil {
		return err
	}
	minerLocalAddr, err := address.NewFromString(th.TestAddress2)
	if err != nil {
		return err
	}

	log.Println("\t[miner] creating miner")
	var wg sync.WaitGroup
	wg.Add(1)
	var minerAddr address.Address
	go func() {
		peer := core.RequireRandomPeerID()
		var err error
		minerAddr, err = fc.Miner().Create(ctx, minerLocalAddr, types.NewBytesAmount(100000), peer, types.NewAttoFILFromFIL(400))
		if err != nil {
			panic(errors.Wrap(err, "failed to create miner"))
		}
		wg.Done()
	}()

	if _, err := fc.Mpool().View(ctx, 1); err != nil {
		return errors.Wrap(err, "failed to wait for message pool")
	}
	if _, err := fc.Mining().Once(ctx); err != nil {
		return errors.Wrap(err, "failed to mine")
	}

	wg.Wait()

	log.Println("\t[miner] adding ask")
	_, err = fc.Miner().AddAsk(ctx, minerLocalAddr, minerAddr, types.NewBytesAmount(1000), types.NewAttoFILFromFIL(10))
	if err != nil {
		return errors.Wrap(err, "failed to add ask")
	}

	if _, err := fc.Mining().Once(ctx); err != nil {
		return errors.Wrap(err, "failed to mine")
	}

	log.Println("\t[client] adding bid")
	if _, err := fc.Client().AddBid(ctx, clientAddr, types.NewBytesAmount(10), types.NewAttoFILFromFIL(9)); err != nil {
		return errors.Wrap(err, "failed to add bid")
	}

	if _, err := fc.Mining().Once(ctx); err != nil {
		return errors.Wrap(err, "failed to mine")
	}

	// TODO: what about deals?
	// They require to have two different nodes, so probably want something like a --miner and a --client flag here
	// eventually. For now the above gives us sth to work with.
	//
	//
	// log.Println("\t[client] importing data")
	// nd, err := fc.Client().ImportData(ctx, strings.NewReader("Some text!\n"))
	// if err != nil {
	// 	return errors.Wrap(err, "failed to import data")
	// }
	// log.Println("\t[client] adding deal")
	// if _, err := fc.Client().ProposeDeal(ctx, 0, 0, nd.Cid()); err != nil {
	// 	return errors.Wrap(err, "failed to propse deal")
	// }

	// if _, err := fc.Mining().Once(ctx); err != nil {
	// 	return errors.Wrap(err, "failed to mine")
	// }

	return nil
}

// readFiles reads a list of file paths and returns them in a format compatible to ipfs-cmdskit.
func readFiles(fpaths []string) (files.File, error) {
	res := make([]files.File, len(fpaths))
	for i, fpath := range fpaths {
		fpath = filepath.ToSlash(filepath.Clean(fpath))

		stat, err := os.Lstat(fpath)
		if err != nil {
			return nil, err
		}

		f, err := files.NewSerialFile(path.Base(fpath), fpath, false, stat)
		if err != nil {
			return nil, err
		}

		res[i] = f
	}

	return files.NewSliceFile("", "", res), nil
}
