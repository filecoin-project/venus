package node

import (
	"context"

	bstore "gx/ipfs/QmTVDM4LCSUMFNQzbDLL9zQwp8usE6QHymFdh3h8vL9v6b/go-ipfs-blockstore"
	"gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	bserv "github.com/ipfs/go-ipfs/blockservice"
	offline "github.com/ipfs/go-ipfs/exchange/offline"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/repo"
)

// FilecoinInit initializes a filecoin node in the given repo
// TODO: accept options?
//  - configurable genesis block
func Init(ctx context.Context, r repo.Repo) error {
	// TODO(ipfs): make the blockstore and blockservice have the same interfaces
	// so that this becomes less painful
	bs := bstore.NewBlockstore(r.Datastore())
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}

	cm := core.NewChainManager(r.Datastore(), cst)
	if err := cm.Genesis(ctx, core.InitGenesis); err != nil {
		return err
	}

	return nil
}
