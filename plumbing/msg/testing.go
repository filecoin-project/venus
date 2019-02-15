package msg

import (
	"context"

	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	offline "gx/ipfs/QmYZwey1thDTynSrvd6qQkX24UpTka6TFhQ2v569UpoqxD/go-ipfs-exchange-offline"
	bserv "gx/ipfs/QmZuPasxd7fSgtzRzCL7Z8J8QwDJML2fgBUExRbQCqb4BT/go-blockservice"
	hamt "gx/ipfs/QmeoCaPwsaPtW34W4vnPEYFYNgNFAygknmX2RRBbGytF9Y/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/wallet"
	"github.com/stretchr/testify/require"
)

type commonDeps struct {
	repo       repo.Repo
	wallet     *wallet.Wallet
	chainStore *chain.DefaultStore
	blockstore bstore.Blockstore
	cst        *hamt.CborIpldStore
}

func requireCommonDeps(require *require.Assertions) *commonDeps { //nolint: deadcode
	return requireCommonDepsWithGif(require, consensus.InitGenesis)
}

func requireCommonDepsWithGif(require *require.Assertions, gif consensus.GenesisInitFunc) *commonDeps {
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
