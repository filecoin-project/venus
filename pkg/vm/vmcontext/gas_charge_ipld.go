package vmcontext

import (
	"github.com/filecoin-project/venus/pkg/vm/gas"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	xerrors "github.com/pkg/errors"
)

var _ cbor.IpldBlockstore = (*GasChargeBlockStore)(nil)

type GasChargeBlockStore struct {
	gasTank   *gas.GasTracker
	pricelist gas.Pricelist
	inner     cbor.IpldBlockstore
}

func (bs *GasChargeBlockStore) Get(c cid.Cid) (blocks.Block, error) {
	bs.gasTank.Charge(bs.pricelist.OnIpldGet(), "storage get %s", c)

	blk, err := bs.inner.Get(c)
	if err != nil {
		panic(xerrors.WithMessage(err, "failed to get block from blockstore"))
	}
	return blk, nil
}

func (bs *GasChargeBlockStore) Put(blk blocks.Block) error {
	bs.gasTank.Charge(bs.pricelist.OnIpldPut(len(blk.RawData())), "%s storage put %d bytes", blk.Cid(), len(blk.RawData()))

	if err := bs.inner.Put(blk); err != nil {
		panic(xerrors.WithMessage(err, "failed to write data to disk"))
	}
	return nil
}
