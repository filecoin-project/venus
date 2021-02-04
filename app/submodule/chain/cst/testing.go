package cst

import (
	"context"
	"testing"

	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/genesis"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/vm/register"
	"github.com/filecoin-project/venus/pkg/wallet"
)

type commonDeps struct {
	repo       repo.Repo
	wallet     *wallet.Wallet
	chainStore *chain.Store
	messages   *chain.MessageStore
	blockstore bstore.Blockstore
	cst        cbor.IpldStore
	chainState *ChainStateReadWriter
}

func requiredCommonDeps(t *testing.T, gif genesis.InitFunc) *commonDeps { // nolint: deadcode
	r := repo.NewInMemoryRepo()
	bs := r.Datastore()
	return requireCommonDepsWithGifAndBlockstore(t, gif, r, bs)
}

// This version is useful if you are installing actors with consensus.AddActor and you
// need to set some actor state up ahead of time (actor state is ultimately found in the
// block store).
func requireCommonDepsWithGifAndBlockstore(t *testing.T, gif genesis.InitFunc, r repo.Repo, bs bstore.Blockstore) *commonDeps {
	cst := cbor.NewCborStore(bs)
	chainStore, err := genesis.Init(context.Background(), r, bs, cst, gif)
	require.NoError(t, err)
	messageStore := chain.NewMessageStore(bs)

	backend, err := wallet.NewDSBackend(r.WalletDatastore(), r.Config().Wallet.PassphraseConfig)
	require.NoError(t, err)
	wallet := wallet.New(backend)
	genBlk, err := chainStore.GetGenesisBlock(context.TODO())
	require.NoError(t, err)
	drand, err := beacon.DrandConfigSchedule(genBlk.Timestamp, r.Config().NetworkParams.BlockDelay, r.Config().NetworkParams.DrandSchedule)
	require.NoError(t, err)
	chainState := NewChainStateReadWriter(chainStore, messageStore, bs, register.DefaultActors, drand)

	return &commonDeps{
		repo:       r,
		wallet:     wallet,
		chainStore: chainStore,
		messages:   messageStore,
		blockstore: bs,
		cst:        cst,
		chainState: chainState,
	}
}
