package slashfilter

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/types"
)

// ISlashFilter used to detect whether the miner mined a invalidated block , support local db and mysql storage
type ISlashFilter interface {
	MinedBlock(ctx context.Context, bh *types.BlockHeader, parentEpoch abi.ChainEpoch) (cid.Cid, bool, error)
}

// LocalSlashFilter use badger db to save mined block for detect slash consensus block
type LocalSlashFilter struct {
	byEpoch   ds.Datastore // double-fork mining faults, parent-grinding fault
	byParents ds.Datastore // time-offset mining faults
}

// NewLocalSlashFilter create a slash filter base on badger db
func NewLocalSlashFilter(dstore ds.Batching) ISlashFilter {
	return &LocalSlashFilter{
		byEpoch:   namespace.Wrap(dstore, ds.NewKey("/slashfilter/epoch")),
		byParents: namespace.Wrap(dstore, ds.NewKey("/slashfilter/parents")),
	}
}

// MinedBlock check whether the block mined is slash
func (f *LocalSlashFilter) MinedBlock(ctx context.Context, bh *types.BlockHeader, parentEpoch abi.ChainEpoch) (cid.Cid, bool, error) {
	epochKey := ds.NewKey(fmt.Sprintf("/%s/%d", bh.Miner, bh.Height))
	{
		// double-fork mining (2 blocks at one epoch)
		doubleForkWitness, doubleForkFault, err := checkFault(ctx, f.byEpoch, epochKey, bh, "double-fork mining faults")
		if err != nil {
			return cid.Undef, false, fmt.Errorf("check double-fork mining faults: %w", err)
		}

		if doubleForkFault {
			return doubleForkWitness, doubleForkFault, nil
		}
	}

	parentsKey := ds.NewKey(fmt.Sprintf("/%s/%s", bh.Miner, types.NewTipSetKey(bh.Parents...).String()))
	{
		// time-offset mining faults (2 blocks with the same parents)
		timeOffsetWitness, timeOffsetFault, err := checkFault(ctx, f.byParents, parentsKey, bh, "time-offset mining faults")
		if err != nil {
			return cid.Undef, false, fmt.Errorf("check time-offset mining faults: %w", err)
		}

		if timeOffsetFault {
			return timeOffsetWitness, timeOffsetFault, nil
		}
	}

	{
		// parent-grinding fault (didn't mine on top of our own block)

		// First check if we have mined a block on the parent epoch
		parentEpochKey := ds.NewKey(fmt.Sprintf("/%s/%d", bh.Miner, parentEpoch))
		have, err := f.byEpoch.Has(ctx, parentEpochKey)
		if err != nil {
			return cid.Undef, false, err
		}

		if have {
			// If we had, make sure it's in our parent tipset
			cidb, err := f.byEpoch.Get(ctx, parentEpochKey)
			if err != nil {
				return cid.Undef, false, fmt.Errorf("getting other block cid: %w", err)
			}

			_, parent, err := cid.CidFromBytes(cidb)
			if err != nil {
				return cid.Undef, false, err
			}

			var found bool
			for _, c := range bh.Parents {
				if c.Equals(parent) {
					found = true
				}
			}

			if !found {
				log.Infof("produced block would trigger 'parent-grinding fault' consensus fault; miner: %s; bh: %s, expected parent: %s", bh.Miner, bh.Cid(), parent)
				return parent, true, nil
			}
		}
	}

	if err := f.byParents.Put(ctx, parentsKey, bh.Cid().Bytes()); err != nil {
		return cid.Undef, false, fmt.Errorf("putting byParents entry: %w", err)
	}

	if err := f.byEpoch.Put(ctx, epochKey, bh.Cid().Bytes()); err != nil {
		return cid.Undef, false, fmt.Errorf("putting byEpoch entry: %w", err)
	}

	return cid.Undef, false, nil
}

func checkFault(ctx context.Context, t ds.Datastore, key ds.Key, bh *types.BlockHeader, faultType string) (cid.Cid, bool, error) {
	fault, err := t.Has(ctx, key)
	if err != nil {
		return cid.Undef, false, fmt.Errorf("failed to read from datastore: %w", err)
	}

	if fault {
		cidb, err := t.Get(ctx, key)
		if err != nil {
			return cid.Undef, false, fmt.Errorf("getting other block cid: %w", err)
		}

		_, other, err := cid.CidFromBytes(cidb)
		if err != nil {
			return cid.Undef, false, err
		}

		if other == bh.Cid() {
			return cid.Undef, false, nil
		}

		log.Infof("produced block would trigger '%s' consensus fault; miner: %s; bh: %s, other: %s", faultType, bh.Miner, bh.Cid(), other)
		return other, true, nil
	}

	return cid.Undef, false, nil
}
