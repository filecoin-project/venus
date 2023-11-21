package splitstore

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"
	cid "github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	cbg "github.com/whyrusleeping/cbor-gen"
)

type Visitor interface {
	// return true means first visit, to continue walk
	Visit(cid.Cid) bool
	Len() int
}

type syncVisitor struct {
	cidSet map[cid.Cid]struct{}
	mutex  sync.RWMutex
}

func (v *syncVisitor) Visit(c cid.Cid) bool {
	{
		// check if already visited
		v.mutex.RLock()
		_, ok := v.cidSet[c]
		v.mutex.RUnlock()
		if ok {
			return false
		}
	}

	{
		// mark as visited
		v.mutex.Lock()
		v.cidSet[c] = struct{}{}
		v.mutex.Unlock()
	}

	return true
}

func (v *syncVisitor) Len() int {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	return len(v.cidSet)
}

func (v *syncVisitor) Stop() {
	v.mutex.Lock()
	defer v.mutex.Unlock()
}

func NewSyncVisitor() Visitor {
	return &syncVisitor{
		cidSet: make(map[cid.Cid]struct{}),
	}
}

func WalkChain(ctx context.Context, store blockstore.Blockstore, tipsetKey cid.Cid, v Visitor, deepLimit abi.ChainEpoch) error {
	cst := cbor.NewCborStore(store)
	var tsk types.TipSetKey
	err := cst.Get(ctx, tipsetKey, &tsk)
	if err != nil {
		return fmt.Errorf("get tipsetKey(%s): %w", tipsetKey, err)
	}

	blockToWalk := make(chan cid.Cid, 8)
	objectToWalk := make(chan cid.Cid, 64)
	for _, c := range tsk.Cids() {
		blockToWalk <- c
	}

	defer func() {
		log.Debugf("walk chain visited %d object", v.Len())
	}()

	wg := sync.WaitGroup{}
	go func() {
		wg.Add(1)
		defer wg.Done()
		for c := range objectToWalk {
			err := walkObject(ctx, store, c, v)
			if err != nil {
				log.Warnf("walkObject(%s): %s", c, err)
			}
		}
	}()

	for bCid := range blockToWalk {
		if !v.Visit(bCid) {
			continue
		}

		var b types.BlockHeader
		err := cst.Get(ctx, bCid, &b)
		if err != nil {
			return err
		}

		if b.Height%500 == 0 {
			log.Debugf("walking block(%s, %d)", bCid, b.Height)
		}

		if b.Height >= deepLimit {
			objectToWalk <- b.ParentStateRoot
			objectToWalk <- b.Messages
			objectToWalk <- b.ParentMessageReceipts
		}

		if b.Height == 0 {
			close(blockToWalk)
			if len(b.Parents) != 1 {
				return fmt.Errorf("invalid genesis block seed(%v)", b.Parents)
			}
			objectToWalk <- b.Parents[0]
			close(objectToWalk)
		} else {
			for _, c := range b.Parents {
				blockToWalk <- c
			}
			tskCid, err := types.NewTipSetKey(b.Parents...).Cid()
			if err != nil {
				return err
			}
			objectToWalk <- tskCid
		}

	}

	wg.Wait()
	return nil
}

func walkObject(ctx context.Context, store blockstore.Blockstore, c cid.Cid, v Visitor) error {
	if !v.Visit(c) {
		return nil
	}

	// handle only dag-cbor which is the default cid codec of state types
	if c.Prefix().Codec != cid.DagCBOR {
		return nil
	}

	var links []cid.Cid
	err := store.View(ctx, c, func(data []byte) error {
		return cbg.ScanForLinks(bytes.NewReader(data), func(c cid.Cid) {
			links = append(links, c)
		})
	})

	if err != nil {
		return fmt.Errorf("error scanning linked block (cid: %s): %w", c, err)
	}

	for _, c := range links {
		err := walkObject(ctx, store, c, v)
		if err != nil {
			// try best to walk more object
			log.Warnf("walkObject(%s): %s", c, err)
		}
	}

	return nil
}
