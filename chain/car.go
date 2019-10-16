package chain

import (
	"context"
	"io"

	blocks "github.com/ipfs/go-block-format"
	car "github.com/ipfs/go-car"
	carutil "github.com/ipfs/go-car/util"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/types"
)

var logCar = logging.Logger("chain/car")

type carChainReader interface {
	GetTipSet(types.TipSetKey) (types.TipSet, error)
}
type carMessageReader interface {
	MessageProvider
}

// Export will export a chain (all blocks and their messages) to the writer `out`.
func Export(ctx context.Context, headTS types.TipSet, cr carChainReader, mr carMessageReader, out io.Writer) (types.TipSetKey, error) {
	// fail if headTS isn't in the store.
	if _, err := cr.GetTipSet(headTS.Key()); err != nil {
		return types.UndefTipSet.Key(), err
	}

	// Write the car header
	chb, err := cbor.DumpObject(car.CarHeader{
		Roots:   headTS.Key().ToSlice(),
		Version: 1,
	})
	if err != nil {
		return types.UndefTipSet.Key(), err
	}

	logCar.Debugf("car file chain head: %s", headTS.Key())
	if err := carutil.LdWrite(out, chb); err != nil {
		return types.UndefTipSet.Key(), err
	}

	iter := IterAncestors(ctx, cr, headTS)
	// accumulate TipSets in descending order.
	for ; !iter.Complete(); err = iter.Next() {
		if err != nil {
			return types.UndefTipSet.Key(), err
		}
		tip := iter.Value()
		// write blocks
		for i := 0; i < tip.Len(); i++ {
			hdr := tip.At(i)
			logCar.Debugf("writing block: %s", hdr.Cid())
			if err := carutil.LdWrite(out, hdr.Cid().Bytes(), hdr.ToNode().RawData()); err != nil {
				return types.UndefTipSet.Key(), err
			}

			msgs, err := mr.LoadMessages(ctx, hdr.Messages)
			if err != nil {
				return types.UndefTipSet.Key(), err
			}

			if len(msgs) > 0 {
				logCar.Debugf("writing message collection: %s", hdr.Messages)
				if err := carutil.LdWrite(out, hdr.Messages.Bytes(), types.MessageCollection(msgs).ToNode().RawData()); err != nil {
					return types.UndefTipSet.Key(), err
				}
			}

			// TODO(#3473) we can remove MessageReceipts from the exported file once addressed.
			rect, err := mr.LoadReceipts(ctx, hdr.MessageReceipts)
			if err != nil {
				return types.UndefTipSet.Key(), err
			}

			if len(rect) > 0 {
				logCar.Debugf("writing message-receipt collection: %s", hdr.Messages)
				if err := carutil.LdWrite(out, hdr.MessageReceipts.Bytes(), types.ReceiptCollection(rect).ToNode().RawData()); err != nil {
					return types.UndefTipSet.Key(), err
				}
			}
		}
	}
	return headTS.Key(), nil
}

type carStore interface {
	Put(blocks.Block) error
}

// Import imports a chain from `in` to `bs`.
func Import(ctx context.Context, cs carStore, in io.Reader) (types.TipSetKey, error) {
	header, err := car.LoadCar(cs, in)
	if err != nil {
		return types.UndefTipSet.Key(), err
	}
	headKey := types.NewTipSetKey(header.Roots...)
	return headKey, nil
}
