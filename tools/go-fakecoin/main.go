package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"gx/ipfs/QmaG4DZ4JaqEfvPWt5nPPgoTzhc1tr1T3f4Nu9Jpdm8ymY/go-ipfs-blockstore"
	"gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	bserv "github.com/ipfs/go-ipfs/blockservice"
	offline "github.com/ipfs/go-ipfs/exchange/offline"

	"github.com/filecoin-project/go-filecoin/core"
	repo "github.com/filecoin-project/go-filecoin/repo"
)

var length int
var repodir string

func init() {
	flag.IntVar(&length, "length", 5, "length of fake chain to create")

	// Default repodir is different than Filecoin to avoid accidental clobbering of real data.
	flag.StringVar(&repodir, "repodir", "~/.fakecoin", "repo directory to use")
}

func main() {
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
		cm := getChainManager(r.Datastore())
		err = cm.Load()
		if err != nil {
			log.Fatal(err)
		}
		err = fake(length, fakeDeps{cm.GetBestBlock, cm.ProcessNewBlock})
		if err != nil {
			log.Fatal(err)
		}
	}
	// TODO: Make usage message reflect the command argument.
}

func getChainManager(d repo.Datastore) *core.ChainManager {
	bs := blockstore.NewBlockstore(d)
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	return core.NewChainManager(d, cst)
}

type fakeDeps = struct {
	getBestBlock    core.BestBlockGetter
	processNewBlock core.NewBlockProcessor
}

func fake(length int, deps fakeDeps) error {
	ctx := context.Background()

	blk := deps.getBestBlock()

	_, err := core.AddChain(ctx, deps.processNewBlock, blk, length)
	if err != nil {
		return err
	}
	fmt.Printf("Added chain of %d empty blocks.\n", length)

	return err
}
