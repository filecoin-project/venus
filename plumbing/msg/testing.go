package msg

import (
	"context"

	hamt "gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	bstore "gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	offline "gx/ipfs/QmSz8kAe2JCKp2dWSG8gHSWnwSmne8YfRXTeK5HBmc9L7t/go-ipfs-exchange-offline"
	bserv "gx/ipfs/QmZsGVGCqMCNzHLNMB6q4F6yyvomqf1VxwhJwSfgo1NGaF/go-blockservice"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/wallet"
)

type commonDeps struct {
	repo       repo.Repo
	wallet     *wallet.Wallet
	chainStore *chain.DefaultStore
	blockstore bstore.Blockstore
	cst        *hamt.CborIpldStore
}

func requiredCommonDeps(require *require.Assertions, gif consensus.GenesisInitFunc) *commonDeps { // nolint: deadcode
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	return requireCommonDepsWithGifAndBlockstore(require, gif, r, bs)
}

// This version is useful if you are installing actors with consensus.AddActor and you
// need to set some actor state up ahead of time (actor state is ultimately found in the
// block store).
func requireCommonDepsWithGifAndBlockstore(require *require.Assertions, gif consensus.GenesisInitFunc, r repo.Repo, bs bstore.Blockstore) *commonDeps {
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	chainStore, err := chain.Init(context.Background(), r, bs, cst, gif)
	require.NoError(err)
	backend, err := wallet.NewDSBackend(r.WalletDatastore())
	require.NoError(err)
	wallet := wallet.New(backend)

	return &commonDeps{
		repo:       r,
		wallet:     wallet,
		chainStore: chainStore,
		blockstore: bs,
		cst:        cst,
	}
}
