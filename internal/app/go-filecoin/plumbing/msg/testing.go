package msg

import (
	"context"
	"testing"

	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
)

type commonDeps struct {
	repo       repo.Repo
	wallet     *wallet.Wallet
	chainStore *chain.Store
	messages   *chain.MessageStore
	blockstore bstore.Blockstore
	cst        hamt.CborIpldStore
}

func requiredCommonDeps(t *testing.T, gif consensus.GenesisInitFunc) *commonDeps { // nolint: deadcode
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	return requireCommonDepsWithGifAndBlockstore(t, gif, r, bs)
}

// This version is useful if you are installing actors with consensus.AddActor and you
// need to set some actor state up ahead of time (actor state is ultimately found in the
// block store).
func requireCommonDepsWithGifAndBlockstore(t *testing.T, gif consensus.GenesisInitFunc, r repo.Repo, bs bstore.Blockstore) *commonDeps {
	cst := hamt.CSTFromBstore(bs)
	chainStore, err := chain.Init(context.Background(), r, bs, cst, gif)
	require.NoError(t, err)
	messageStore := chain.NewMessageStore(bs)
	backend, err := wallet.NewDSBackend(r.WalletDatastore())
	require.NoError(t, err)
	wallet := wallet.New(backend)

	return &commonDeps{
		repo:       r,
		wallet:     wallet,
		chainStore: chainStore,
		messages:   messageStore,
		blockstore: bs,
		cst:        cst,
	}
}
