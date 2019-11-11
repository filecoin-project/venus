package export

import (
	"context"
	"io"
	"path/filepath"

	bserv "github.com/ipfs/go-blockservice"
	cid "github.com/ipfs/go-cid"
	badgerds "github.com/ipfs/go-ds-badger"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	dag "github.com/ipfs/go-merkledag"
	errors "github.com/pkg/errors"

	plumbingDag "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/dag"
	block "github.com/filecoin-project/go-filecoin/internal/pkg/block"
	chain "github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	encoding "github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
)

var log = logging.Logger("chain-util/export")

// NewChainExporter returns a ChainExporter.
func NewChainExporter(repoPath string, out io.Writer) (*ChainExporter, error) {
	// open the datastore in readonly mode
	badgerOpt := &badgerds.DefaultOptions
	badgerOpt.ReadOnly = true

	log.Infof("opening filecoin datastore: %s", repoPath)
	badgerPath := filepath.Join(repoPath, "badger/")
	log.Infof("opening badger datastore: %s", badgerPath)
	badgerDS, err := badgerds.NewDatastore(badgerPath, badgerOpt)
	if err != nil {
		return nil, err
	}
	bstore := blockstore.NewBlockstore(badgerDS)
	offl := offline.Exchange(bstore)
	blkserv := bserv.New(bstore, offl)
	dserv := dag.NewDAGService(blkserv)

	chainPath := filepath.Join(repoPath, "chain/")
	log.Infof("opening chain datastore: %s", chainPath)
	chainDS, err := badgerds.NewDatastore(chainPath, badgerOpt)
	if err != nil {
		return nil, err
	}

	headTS, err := getDatastoreHeadTipSet(chainDS, bstore)
	if err != nil {
		return nil, err
	}

	return &ChainExporter{
		bstore:  bstore,
		dagserv: dserv,
		Head:    headTS,
		out:     out,
	}, nil
}

// ChainExporter exports the blockchain contained in bstore to the out writer.
type ChainExporter struct {
	// Where the chain data is kept
	bstore blockstore.Blockstore
	// Makes traversal easier
	dagserv format.DAGService
	// the Head of the chain being exported
	Head block.TipSet
	// Where the chain data is exported to
	out io.Writer
}

// Export will export a chain (all blocks and their messages) to the writer `out`.
func (ce *ChainExporter) Export(ctx context.Context) error {
	msgStore := chain.NewMessageStore(ce.bstore)
	return chain.Export(ctx, ce.Head, ce, msgStore, ce, ce.out)
}

// GetTipSet gets the TipSet for a given TipSetKey from the ChainExporter blockstore.
func (ce *ChainExporter) GetTipSet(key block.TipSetKey) (block.TipSet, error) {
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

// ChainStateTree returns the state tree as a slice of IPLD nodes at the passed stateroot cid `c`.
func (ce *ChainExporter) ChainStateTree(ctx context.Context, c cid.Cid) ([]format.Node, error) {
	return plumbingDag.NewDAG(ce.dagserv).RecursiveGet(ctx, c)
}

func getDatastoreHeadTipSet(ds *badgerds.Datastore, bs blockstore.Blockstore) (block.TipSet, error) {
	bb, err := ds.Get(chain.HeadKey)
	if err != nil {
		return block.UndefTipSet, errors.Wrap(err, "failed to read HeadKey")
	}
	var cids block.TipSetKey
	err = encoding.Decode(bb, &cids)
	if err != nil {
		return block.UndefTipSet, errors.Wrap(err, "failed to cast headCids")
	}
	var blks []*block.Block
	for _, c := range cids.ToSlice() {
		bsBlk, err := bs.Get(c)
		if err != nil {
			return block.UndefTipSet, errors.Wrapf(err, "failed to read key %s from blockstore", c)
		}
		blk, err := block.DecodeBlock(bsBlk.RawData())
		if err != nil {
			return block.UndefTipSet, errors.Wrapf(err, "failed to decode block %s", c)
		}
		blks = append(blks, blk)
	}
	headTS, err := block.NewTipSet(blks...)
	if err != nil {
		return block.UndefTipSet, errors.Wrap(err, "failed to create Head tipset from headkey")
	}
	return headTS, nil
}
