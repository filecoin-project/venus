package blockstore

import (
	"context"
	"errors"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
)

type ChainIO interface {
	ChainReadObj(context.Context, cid.Cid) ([]byte, error)
	ChainHasObj(context.Context, cid.Cid) (bool, error)
}

type apiBlockstore struct {
	api ChainIO
}

// This blockstore is adapted in the constructor.
var _ BasicBlockstore = (*apiBlockstore)(nil)

func NewAPIBlockstore(cio ChainIO) Blockstore {
	bs := &apiBlockstore{api: cio}
	return Adapt(bs) // return an adapted blockstore.
}

func (a *apiBlockstore) DeleteBlock(context.Context, cid.Cid) error {
	return errors.New("not supported")
}

func (a *apiBlockstore) Has(ctx context.Context, c cid.Cid) (bool, error) {
	return a.api.ChainHasObj(ctx, c)
}

func (a *apiBlockstore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	bb, err := a.api.ChainReadObj(ctx, c)
	if err != nil {
		return nil, err
	}
	return blocks.NewBlockWithCid(bb, c)
}

func (a *apiBlockstore) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	bb, err := a.api.ChainReadObj(ctx, c)
	if err != nil {
		return 0, err
	}
	return len(bb), nil
}

func (a *apiBlockstore) Put(context.Context, blocks.Block) error {
	return errors.New("not supported")
}

func (a *apiBlockstore) PutMany(context.Context, []blocks.Block) error {
	return errors.New("not supported")
}

func (a *apiBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return nil, errors.New("not supported")
}

func (a *apiBlockstore) HashOnRead(enabled bool) {
}
