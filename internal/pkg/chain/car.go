package chain

import (
	"context"
	"io"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-car"
	carutil "github.com/ipld/go-car/util"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

var logCar = logging.Logger("chain/car")

type carChainReader interface {
	GetTipSet(block.TipSetKey) (*block.TipSet, error)
}
type carMessageReader interface {
	MessageProvider
}

type carStateReader interface {
	ChainStateTree(ctx context.Context, c cid.Cid) ([]format.Node, error)
}

// Fields need to stay lower case to match car's default refmt encoding as
// refmt can't handle arbitrary casing.
type carHeader struct {
	Roots   block.TipSetKey `cbor:"roots"`
	Version uint64          `cbor:"version"`
}

// Export will export a chain (all blocks and their messages) to the writer `out`.
func Export(ctx context.Context, headTS *block.TipSet, cr carChainReader, mr carMessageReader, sr carStateReader, out io.Writer) error {
	// ensure we don't duplicate writes to the car file. // e.g. only write EmptyMessageCID once.
	filter := make(map[cid.Cid]bool)

	// fail if headTS isn't in the store.
	if _, err := cr.GetTipSet(headTS.Key()); err != nil {
		return err
	}

	// Write the car header
	ch := carHeader{
		Roots:   headTS.Key(),
		Version: 1,
	}
	chb, err := encoding.Encode(ch)
	if err != nil {
		return err
	}

	logCar.Debugf("car file chain head: %s", headTS.Key())
	if err := carutil.LdWrite(out, chb); err != nil {
		return err
	}

	iter := IterAncestors(ctx, cr, headTS)
	// accumulate TipSets in descending order.
	for ; !iter.Complete(); err = iter.Next() {
		if err != nil {
			return err
		}
		tip := iter.Value()
		// write blocks
		for i := 0; i < tip.Len(); i++ {
			hdr := tip.At(i)
			logCar.Debugf("writing block: %s", hdr.Cid())

			if !filter[hdr.Cid()] {
				if err := carutil.LdWrite(out, hdr.Cid().Bytes(), hdr.ToNode().RawData()); err != nil {
					return err
				}
				filter[hdr.Cid()] = true
			}

			meta, err := mr.LoadTxMeta(ctx, hdr.Messages.Cid)
			if err != nil {
				return err
			}

			if !filter[hdr.Messages.Cid] {
				logCar.Debugf("writing txMeta: %s", hdr.Messages)
				if err := exportTxMeta(ctx, out, meta); err != nil {
					return err
				}
				filter[hdr.Messages.Cid] = true
			}

			secpMsgs, blsMsgs, err := mr.LoadMetaMessages(ctx, hdr.Messages.Cid)
			if err != nil {
				return err
			}

			if !filter[meta.SecpRoot.Cid] {
				logCar.Debugf("writing secp message collection: %s", hdr.Messages)
				if err := exportAMTSignedMessages(ctx, out, secpMsgs); err != nil {
					return err
				}
				filter[meta.SecpRoot.Cid] = true
			}

			if !filter[meta.BLSRoot.Cid] {
				logCar.Debugf("writing bls message collection: %s", hdr.Messages)
				if err := exportAMTUnsignedMessages(ctx, out, blsMsgs); err != nil {
					return err
				}
				filter[meta.BLSRoot.Cid] = true
			}

			// TODO(#3473) we can remove MessageReceipts from the exported file once addressed.
			rect, err := mr.LoadReceipts(ctx, hdr.MessageReceipts.Cid)
			if err != nil {
				return err
			}

			if !filter[hdr.MessageReceipts.Cid] {
				logCar.Debugf("writing message-receipt collection: %s", hdr.Messages)
				if err := exportAMTReceipts(ctx, out, rect); err != nil {
					return err
				}
				filter[hdr.MessageReceipts.Cid] = true
			}

			if hdr.Height == 0 {
				logCar.Debugf("writing state tree: %s", hdr.StateRoot)
				stateRoots, err := sr.ChainStateTree(ctx, hdr.StateRoot.Cid)
				if err != nil {
					return err
				}
				for _, r := range stateRoots {
					if err := carutil.LdWrite(out, r.Cid().Bytes(), r.RawData()); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func exportAMTSignedMessages(ctx context.Context, out io.Writer, smsgs []*types.SignedMessage) error {
	ms := carWritingMessageStore(out)

	cids, err := ms.storeSignedMessages(smsgs)
	if err != nil {
		return err
	}

	_, err = ms.storeAMTCids(ctx, cids)
	return err
}

func exportAMTUnsignedMessages(ctx context.Context, out io.Writer, umsgs []*types.UnsignedMessage) error {
	ms := carWritingMessageStore(out)

	cids, err := ms.storeUnsignedMessages(umsgs)
	if err != nil {
		return err
	}

	_, err = ms.storeAMTCids(ctx, cids)
	return err
}

func exportAMTReceipts(ctx context.Context, out io.Writer, receipts []types.MessageReceipt) error {
	ms := carWritingMessageStore(out)

	_, err := ms.StoreReceipts(ctx, receipts)
	return err
}

func exportTxMeta(ctx context.Context, out io.Writer, meta types.TxMeta) error {
	ms := carWritingMessageStore(out)
	_, err := ms.StoreTxMeta(ctx, meta)
	return err
}

func carWritingMessageStore(out io.Writer) *MessageStore {
	return NewMessageStore(carExportBlockstore{out: out})
}

type carStore interface {
	Put(blocks.Block) error
}

// Import imports a chain from `in` to `bs`.
func Import(ctx context.Context, cs carStore, in io.Reader) (block.TipSetKey, error) {
	header, err := car.LoadCar(cs, in)
	if err != nil {
		return block.TipSetKey{}, err
	}
	headKey := block.NewTipSetKey(header.Roots...)
	return headKey, nil
}

// carExportBlockstore allows a structure that would normally put blocks in a block store to output to a car file instead.
type carExportBlockstore struct {
	out io.Writer
}

func (cs carExportBlockstore) DeleteBlock(c cid.Cid) error         { panic("not implement") }
func (cs carExportBlockstore) Has(c cid.Cid) (bool, error)         { panic("not implement") }
func (cs carExportBlockstore) Get(c cid.Cid) (blocks.Block, error) { panic("not implement") }
func (cs carExportBlockstore) GetSize(c cid.Cid) (int, error)      { panic("not implement") }
func (cs carExportBlockstore) Put(b blocks.Block) error {
	return carutil.LdWrite(cs.out, b.Cid().Bytes(), b.RawData())
}
func (cs carExportBlockstore) PutMany(b []blocks.Block) error { panic("not implement") }
func (cs carExportBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	panic("not implement")
}
func (cs carExportBlockstore) HashOnRead(enabled bool) { panic("not implement") }
