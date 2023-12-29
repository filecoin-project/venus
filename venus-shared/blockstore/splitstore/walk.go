package splitstore

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"
	cid "github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	cbg "github.com/whyrusleeping/cbor-gen"
)

type Visitor interface {
	// Visit return true means first visit, to continue walk
	// it may be called concurrently
	Visit(cid.Cid) bool
	HandleError(cid.Cid, error)
	Len() int
	Cids() []cid.Cid
	RegisterVisitHook(func(cid.Cid) bool)
	RegisterErrorHook(func(cid.Cid, error))
}

type syncVisitor struct {
	cidSet     map[cid.Cid]struct{}
	mutex      sync.RWMutex
	visitHooks []func(cid.Cid) bool
	errorHooks []func(cid.Cid, error)
}

var _ Visitor = (*syncVisitor)(nil)

func (v *syncVisitor) Visit(c cid.Cid) bool {
	{
		// check if already visited
		v.mutex.RLock()
		_, ok := v.cidSet[c]
		v.mutex.RUnlock()
		if ok {
			return false
		}
		for _, hook := range v.visitHooks {
			if !hook(c) {
				return false
			}
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

func (v *syncVisitor) HandleError(c cid.Cid, err error) {
	for _, hook := range v.errorHooks {
		hook(c, err)
	}
}

func (v *syncVisitor) Len() int {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	return len(v.cidSet)
}

func (v *syncVisitor) Cids() []cid.Cid {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	cids := make([]cid.Cid, 0, len(v.cidSet))
	for c := range v.cidSet {
		cids = append(cids, c)
	}
	return cids
}

func (v *syncVisitor) RegisterVisitHook(hook func(cid.Cid) bool) {
	v.visitHooks = append(v.visitHooks, hook)
}

func (v *syncVisitor) RegisterErrorHook(hook func(cid.Cid, error)) {
	v.errorHooks = append(v.errorHooks, hook)
}

type Flag string

const (
	FlagEnableDefaultErrorHandler Flag = "enable-default-error-handler"
)

func NewSyncVisitor(flags ...Flag) Visitor {
	v := &syncVisitor{
		cidSet: make(map[cid.Cid]struct{}),
	}

	for _, flag := range flags {
		switch flag {
		case FlagEnableDefaultErrorHandler:
			v.RegisterErrorHook(func(c cid.Cid, err error) {
				log.Debugf("visit %s fail: %s", c, err)
			})
		default:
			log.Warnf("unknown flag: %s", flag)
		}
	}

	return v
}

func WalkUntil(ctx context.Context, store blockstore.Blockstore, tipsetKey cid.Cid, targetEpoch abi.ChainEpoch) error {
	log.Info("walk chain to sync state")
	v := NewSyncVisitor()
	return walkChain(ctx, store, tipsetKey, v, targetEpoch, true)
}

func WalkHeader(ctx context.Context, store blockstore.Blockstore, tipsetKey cid.Cid, targetEpoch abi.ChainEpoch) error {
	log.Info("walk chain to back up header")
	v := NewSyncVisitor()
	return walkChain(ctx, store, tipsetKey, v, targetEpoch, false)
}

func walkChain(ctx context.Context, store blockstore.Blockstore, tipsetKey cid.Cid, v Visitor, targetEpoch abi.ChainEpoch, walkState bool) (err error) {
	skipMessage := false
	skipReceipts := false

	start := time.Now()
	defer func() {
		log.Infow("finish walk chain", "from", tipsetKey, "target", targetEpoch, "elapsed", time.Since(start), "visited", v.Len(), "error", err)
	}()

	log.Infow("start walk chain", "from", tipsetKey, "target", targetEpoch)

	cst := cbor.NewCborStore(store)
	var tsk types.TipSetKey
	err = cst.Get(ctx, tipsetKey, &tsk)
	if err != nil {
		err = fmt.Errorf("get tipsetKey(%s): %w", tipsetKey, err)
		return
	}
	var b types.BlockHeader
	err = cst.Get(ctx, tsk.Cids()[0], &b)
	if err != nil {
		err = fmt.Errorf("get block(%s): %w", tsk.Cids()[0], err)
		return
	}

	blockToWalk := make([]cid.Cid, 0)
	objectToWalk := make([]cid.Cid, 0)

	pushObject := func(c ...cid.Cid) {
		if !walkState {
			return
		}
		objectToWalk = append(objectToWalk, c...)
	}

	blockToWalk = append(blockToWalk, tsk.Cids()...)

	blockCount := 0
	for len(blockToWalk) > 0 {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		bCid := blockToWalk[0]
		blockToWalk = blockToWalk[1:]

		if !v.Visit(bCid) {
			continue
		}

		var b types.BlockHeader
		err = cst.Get(ctx, bCid, &b)
		if err != nil {
			return err
		}

		if b.Height%1000 == 0 {
			log.Debugf("walking block(%s, %d)", bCid, b.Height)
		}

		if b.Height < targetEpoch || b.Height == 0 {
			if b.Height == 0 {
				if len(b.Parents) != 1 {
					err = fmt.Errorf("invalid genesis block seed(%v)", b.Parents)
					return
				}
				// genesis block
				objectToWalk = append(objectToWalk, b.Parents[0])
			}
		} else {
			if !skipMessage {
				pushObject(b.Messages)
			}
			if !skipReceipts {
				pushObject(b.ParentMessageReceipts)
			}

			pushObject(b.ParentStateRoot)

			blockToWalk = append(blockToWalk, b.Parents...)
			var tskCid cid.Cid
			tskCid, err = types.NewTipSetKey(b.Parents...).Cid()
			if err != nil {
				return err
			}

			// objectToWalk = append(objectToWalk, tskCid)
			pushObject(tskCid)
		}
		blockCount++
	}

	log.Infof("walk chain visited %d block header, remain %d object to walk", blockCount, len(objectToWalk))

	objectCh := make(chan cid.Cid, len(objectToWalk))
	for _, c := range objectToWalk {
		objectCh <- c
	}
	close(objectCh)

	wg := sync.WaitGroup{}
	walkObjectWorkerCount := runtime.NumCPU() / 2
	log.Debugf("start %d walk object worker", walkObjectWorkerCount)
	wg.Add(walkObjectWorkerCount)
	for i := 0; i < walkObjectWorkerCount; i++ {
		go func() {
			defer wg.Done()
			for c := range objectCh {
				walkObject(ctx, store, c, v)
			}
		}()
	}
	wg.Wait()

	return nil
}

func walkObject(ctx context.Context, store blockstore.Blockstore, c cid.Cid, v Visitor) {
	if !v.Visit(c) {
		return
	}

	// handle only dag-cbor which is the default cid codec of state types
	if c.Prefix().Codec != cid.DagCBOR {
		// should be exit
		has, err := store.Has(ctx, c)
		if err != nil {
			v.HandleError(c, fmt.Errorf("check has(%s): %w", c, err))
		}
		if !has {
			v.HandleError(c, fmt.Errorf("object(%s) not found", c))
		}
		return
	}

	var links []cid.Cid
	err := store.View(ctx, c, func(data []byte) error {
		return cbg.ScanForLinks(bytes.NewReader(data), func(c cid.Cid) {
			links = append(links, c)
		})
	})
	if err != nil {
		v.HandleError(c, fmt.Errorf("scan link for(cid: %s): %w", c, err))
	}

	for _, c := range links {
		walkObject(ctx, store, c, v)
	}

}

func backupHeader(ctx context.Context, from cid.Cid, src, dst blockstore.Blockstore) error {
	log.Infow("backup header", "from", from)
	v := NewSyncVisitor(FlagEnableDefaultErrorHandler)
	v.RegisterVisitHook(func(c cid.Cid) bool {
		has, err := dst.Has(ctx, c)
		if err != nil {
			log.Warnf("backup header: check %s whether exit in destination fail", c)
		}
		return !has
	})

	return walkChain(ctx, src, from, v, 1, false)
}

func Once(f func()) func() {
	var once sync.Once
	return func() {
		once.Do(f)
	}
}
