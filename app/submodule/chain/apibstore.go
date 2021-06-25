package chain

import (
	"context"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"golang.org/x/xerrors"
)

type ChainIO interface { //nolint
	ChainReadObj(context.Context, cid.Cid) ([]byte, error)
	ChainHasObj(context.Context, cid.Cid) (bool, error)
}

type apiBStore struct {
	api ChainIO
}

// NewAPIBlockstore create new blockstore api
func NewAPIBlockstore(cio ChainIO) blockstore.Blockstore {
	return &apiBStore{
		api: cio,
	}
}

// DeleteBlock implements Blockstore.DeleteBlock.
func (a *apiBStore) DeleteBlock(cid.Cid) error {
	return xerrors.New("not supported")
}

// Has implements Blockstore.Has.
func (a *apiBStore) Has(c cid.Cid) (bool, error) {
	return a.api.ChainHasObj(context.TODO(), c)
}

// Get implements Blockstore.Get.
func (a *apiBStore) Get(c cid.Cid) (blocks.Block, error) {
	bb, err := a.api.ChainReadObj(context.TODO(), c)
	if err != nil {
		return nil, err
	}
	return blocks.NewBlockWithCid(bb, c)
}

// GetSize implements Blockstore.GetSize.
func (a *apiBStore) GetSize(c cid.Cid) (int, error) {
	bb, err := a.api.ChainReadObj(context.TODO(), c)
	if err != nil {
		return 0, err
	}
	return len(bb), nil
}

// Put implements Blockstore.Put.
func (a *apiBStore) Put(blocks.Block) error {
	return xerrors.New("not supported")
}

// PutMany implements Blockstore.PutMany.
func (a *apiBStore) PutMany([]blocks.Block) error {
	return xerrors.New("not supported")
}

// AllKeysChan implements Blockstore.AllKeysChan.
func (a *apiBStore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return nil, xerrors.New("not supported")
}

// HashOnRead implements Blockstore.HashOnRead.
func (a *apiBStore) HashOnRead(enabled bool) {}

var _ blockstore.Blockstore = &apiBStore{}
