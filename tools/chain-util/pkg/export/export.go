package export

import (
	"context"
	"io"

	bserv "github.com/ipfs/go-blockservice"
	car "github.com/ipfs/go-car"
	carutil "github.com/ipfs/go-car/util"
	cid "github.com/ipfs/go-cid"
	badgerds "github.com/ipfs/go-ds-badger"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	cbor "github.com/ipfs/go-ipld-cbor"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	dag "github.com/ipfs/go-merkledag"
	errors "github.com/pkg/errors"

	block "github.com/filecoin-project/go-filecoin/block"
	types "github.com/filecoin-project/go-filecoin/types"
)

var log = logging.Logger("chain-util/export")

func init() {
	logging.SetAllLoggers(logging.LevelInfo)
}

// NewChainExporter returns a ChainExporter.
func NewChainExporter(repoPath string, out io.Writer) (*ChainExporter, error) {
	// open the datastore in readonly mode
	badgerOpt := &badgerds.DefaultOptions
	badgerOpt.ReadOnly = true

	log.Infof("opening chain datastore: %s", repoPath)
	ds, err := badgerds.NewDatastore(repoPath, badgerOpt)
	if err != nil {
		return nil, err
	}

	bstore := blockstore.NewBlockstore(ds)
	offl := offline.Exchange(bstore)
	blkserv := bserv.New(bstore, offl)
	dserv := dag.NewDAGService(blkserv)
	return &ChainExporter{
		bstore:  bstore,
		dagserv: dserv,
		out:     out,
	}, nil
}

// ChainExporter exports the blockchain contained in bstore to the out writer.
type ChainExporter struct {
	// Where the chain data is kept
	bstore blockstore.Blockstore
	// Makes traversal easier
	dagserv format.DAGService
	// Where the chain data is exported to
	out io.Writer
}

// Export will export a chain (all blocks and their messages) to the writer `out`.
func (ce *ChainExporter) Export(headKey block.TipSetKey) error {
	// ensure we don't duplicate writes to the car file. // e.g. only write EmptyMessageCID once.
	filter := make(map[cid.Cid]bool)

	// fail if headTS isn't in the store.
	headTS, err := ce.LoadTipSet(headKey)
	if err != nil {
		return err
	}

	// Write the car header
	chb, err := cbor.DumpObject(car.CarHeader{
		Roots:   headKey.ToSlice(),
		Version: 1,
	})
	if err != nil {
		return err
	}

	log.Infof("car file chain head: %s", headKey)
	if err := carutil.LdWrite(ce.out, chb); err != nil {
		return err
	}

	iter := NewIterator(headTS, ce)
	// accumulate TipSets in descending order.
	for ; !iter.Complete(); err = iter.Next() {
		if err != nil {
			return err
		}
		tip := iter.Value()
		// write blocks
		for i := 0; i < tip.Len(); i++ {
			hdr := tip.At(i)
			log.Debugf("writing block: %s", hdr.Cid())

			if !filter[hdr.Cid()] {
				if err := carutil.LdWrite(ce.out, hdr.Cid().Bytes(), hdr.ToNode().RawData()); err != nil {
					return err
				}
				filter[hdr.Cid()] = true
			}

			secpMsgs, blsMsgs, err := ce.LoadMessages(hdr.Messages.SecpRoot, hdr.Messages.BLSRoot)
			if err != nil {
				return err
			}

			if !filter[hdr.Messages.SecpRoot] {
				log.Debugf("writing message collection: %s", hdr.Messages)
				if err := carutil.LdWrite(ce.out, hdr.Messages.SecpRoot.Bytes(), types.SignedMessageCollection(secpMsgs).ToNode().RawData()); err != nil {
					return err
				}
				filter[hdr.Messages.SecpRoot] = true
			}

			if !filter[hdr.Messages.BLSRoot] {
				log.Debugf("writing message collection: %s", hdr.Messages)
				if err := carutil.LdWrite(ce.out, hdr.Messages.BLSRoot.Bytes(), types.MessageCollection(blsMsgs).ToNode().RawData()); err != nil {
					return err
				}
				filter[hdr.Messages.BLSRoot] = true
			}

			if !filter[hdr.MessageReceipts] {
				// TODO(#3473) we can remove MessageReceipts from the exported file once addressed.
				rect, err := ce.LoadReceipts(hdr.MessageReceipts)
				if err != nil {
					return err
				}

				log.Debugf("writing message-receipt collection: %s", hdr.Messages)
				if err := carutil.LdWrite(ce.out, hdr.MessageReceipts.Bytes(), types.ReceiptCollection(rect).ToNode().RawData()); err != nil {
					return err
				}
				filter[hdr.MessageReceipts] = true
			}

			if hdr.Height == 0 {
				log.Debugf("writing StateRoot: %s", hdr.StateRoot)
				if err := ce.PersistStateTree(hdr.StateRoot); err != nil {
					return err
				}
			}

		}
	}
	return nil
}

// LoadTipSet loads all the TipSet for a given TipSetKey.
func (ce *ChainExporter) LoadTipSet(key block.TipSetKey) (block.TipSet, error) {
	var blks []*block.Block
	for it := key.Iter(); !it.Complete(); it.Next() {
		bsBlk, err := ce.bstore.Get(it.Value())
		if err != nil {
			return block.UndefTipSet, errors.Wrapf(err, "failed to load tipset %s", key.String())
		}
		blk, err := block.DecodeBlock(bsBlk.RawData())
		if err != nil {
			return block.UndefTipSet, errors.Wrapf(err, "failed to decode tipset %s", key.String())
		}
		blks = append(blks, blk)
	}
	return block.NewTipSet(blks...)
}

// LoadMessages loads secp and bls messages from the store.
func (ce *ChainExporter) LoadMessages(secpCid, blsCid cid.Cid) ([]*types.SignedMessage, []*types.UnsignedMessage, error) {
	blk, err := ce.bstore.Get(secpCid)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to load secp blocks %s", blsCid.String())
	}
	secp, err := types.DecodeSignedMessages(blk.RawData())
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to decode secp blocks %s", blsCid.String())
	}
	blk, err = ce.bstore.Get(blsCid)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to load bls blocks %s", blsCid.String())
	}
	bls, err := types.DecodeMessages(blk.RawData())
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to decode bls blocks %s", blsCid.String())
	}
	return secp, bls, nil
}

// LoadReceipts loads messages receipts from the store.
func (ce *ChainExporter) LoadReceipts(c cid.Cid) ([]*types.MessageReceipt, error) {
	blk, err := ce.bstore.Get(c)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load receipts %s", c.String())
	}
	out, err := types.DecodeReceipts(blk.RawData())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode receipts %s", c.String())
	}
	return out, nil
}

// PersistStateTree persist the state tree at `c` to the store.
func (ce *ChainExporter) PersistStateTree(c cid.Cid) error {
	dagNd, err := ce.dagserv.Get(context.TODO(), c)
	if err != nil {
		return errors.Wrapf(err, "failed to load stateroot from dagservice %s", c.String())
	}
	if err := carutil.LdWrite(ce.out, dagNd.Cid().Bytes(), dagNd.RawData()); err != nil {
		return err
	}
	seen := cid.NewSet()
	for _, l := range dagNd.Links() {
		if err := dag.Walk(context.TODO(), ce.enumGetLinks, l.Cid, seen.Visit); err != nil {
			return err
		}
	}
	return nil
}

func (ce *ChainExporter) enumGetLinks(ctx context.Context, c cid.Cid) ([]*format.Link, error) {
	nd, err := ce.dagserv.Get(ctx, c)
	if err != nil {
		return nil, err
	}

	if err := carutil.LdWrite(ce.out, nd.Cid().Bytes(), nd.RawData()); err != nil {
		return nil, err
	}

	return nd.Links(), nil
}

// NewIterator returns a ChainIterator.
func NewIterator(start block.TipSet, loader TipSetLoader) *ChainIterator {
	return &ChainIterator{loader, start}
}

// ChainIterator is an iterator over a collection of TipSets
type ChainIterator struct {
	store TipSetLoader
	value block.TipSet
}

// Value returns the iterator's current value, if not Complete().
func (it *ChainIterator) Value() block.TipSet {
	return it.value
}

// Complete tests whether the iterator is exhausted.
func (it *ChainIterator) Complete() bool {
	return !it.value.Defined()
}

// Next advances the iterator to the next value.
func (it *ChainIterator) Next() error {
	parentKey, err := it.value.Parents()
	// Parents is empty (without error) for the genesis tipset.
	if err != nil || parentKey.Len() == 0 {
		it.value = block.UndefTipSet
	} else {
		it.value, err = it.store.LoadTipSet(parentKey)
	}
	return err
}

// TipSetLoader defines an interface for loading TipSets (accepted by the ChainIterator).
type TipSetLoader interface {
	LoadTipSet(key block.TipSetKey) (block.TipSet, error)
}
