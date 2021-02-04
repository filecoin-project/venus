package events

import (
	"context"
	"github.com/filecoin-project/venus/pkg/block"
	"sync"

	"github.com/filecoin-project/go-state-types/abi"
	"golang.org/x/xerrors"
)

type tsCacheAPI interface {
	ChainGetTipSetByHeight(context.Context, abi.ChainEpoch, block.TipSetKey) (*block.TipSet, error)
	ChainHead(context.Context) (*block.TipSet, error)
}

// tipSetCache implements a simple ring-buffer cache to keep track of recent
// tipsets
type tipSetCache struct {
	mu sync.RWMutex

	cache []*block.TipSet
	start int
	len   int

	storage tsCacheAPI
}

func newTSCache(cap abi.ChainEpoch, storage tsCacheAPI) *tipSetCache {
	return &tipSetCache{
		cache: make([]*block.TipSet, cap),
		start: 0,
		len:   0,

		storage: storage,
	}
}

func (tsc *tipSetCache) add(ts *block.TipSet) error {
	tsc.mu.Lock()
	defer tsc.mu.Unlock()
	if tsc.len > 0 {
		if tsc.cache[tsc.start].EnsureHeight() >= ts.EnsureHeight() {
			return xerrors.Errorf("tipSetCache.add: expected new tipset height to be at least %d, was %d", tsc.cache[tsc.start].EnsureHeight()+1, ts.EnsureHeight())
		}
	}
	tsH, err := ts.Height()
	if err != nil {
		return err
	}
	nextH := tsH

	if tsc.len > 0 {
		tssH, err := tsc.cache[tsc.start].Height()
		if err != nil {
			return err
		}
		nextH = tssH + 1
	}

	// fill null blocks
	for nextH != tsH {
		tsc.start = normalModulo(tsc.start+1, len(tsc.cache))
		tsc.cache[tsc.start] = nil
		if tsc.len < len(tsc.cache) {
			tsc.len++
		}
		nextH++
	}

	tsc.start = normalModulo(tsc.start+1, len(tsc.cache))
	tsc.cache[tsc.start] = ts
	if tsc.len < len(tsc.cache) {
		tsc.len++
	}
	return nil
}

func (tsc *tipSetCache) revert(ts *block.TipSet) error {
	tsc.mu.Lock()
	defer tsc.mu.Unlock()

	return tsc.revertUnlocked(ts)
}

func (tsc *tipSetCache) revertUnlocked(ts *block.TipSet) error {
	if tsc.len == 0 {
		return nil // this can happen, and it's fine
	}

	if !tsc.cache[tsc.start].Equals(ts) {
		return xerrors.New("tipSetCache.revert: revert tipset didn't match cache head")
	}

	tsc.cache[tsc.start] = nil
	tsc.start = normalModulo(tsc.start-1, len(tsc.cache))
	tsc.len--

	_ = tsc.revertUnlocked(nil) // revert null block gap
	return nil
}

func (tsc *tipSetCache) getNonNull(height abi.ChainEpoch) (*block.TipSet, error) {
	for {
		ts, err := tsc.get(height)
		if err != nil {
			return nil, err
		}
		if ts != nil {
			return ts, nil
		}
		height++
	}
}

func (tsc *tipSetCache) get(height abi.ChainEpoch) (*block.TipSet, error) {
	tsc.mu.RLock()

	if tsc.len == 0 {
		tsc.mu.RUnlock()
		log.Warnf("tipSetCache.get: cache is empty, requesting from storage (h=%d)", height)
		return tsc.storage.ChainGetTipSetByHeight(context.TODO(), height, block.EmptyTSK)
	}

	headH, err := tsc.cache[tsc.start].Height()
	if err != nil {
		return nil, err
	}
	if height > headH {
		tsc.mu.RUnlock()
		return nil, xerrors.Errorf("tipSetCache.get: requested tipset not in cache (req: %d, cache head: %d)", height, headH)
	}

	clen := len(tsc.cache)
	var tail *block.TipSet
	for i := 1; i <= tsc.len; i++ {
		tail = tsc.cache[normalModulo(tsc.start-tsc.len+i, clen)]
		if tail != nil {
			break
		}
	}
	tailH, err := tail.Height()
	if err != nil {
		return nil, err
	}
	if height < tailH {
		tsc.mu.RUnlock()
		log.Warnf("tipSetCache.get: requested tipset not in cache, requesting from storage (h=%d; tail=%d;)", height, tail.EnsureHeight())
		return tsc.storage.ChainGetTipSetByHeight(context.TODO(), height, tail.Key())
	}

	ts := tsc.cache[normalModulo(tsc.start-int(headH-height), clen)]
	tsc.mu.RUnlock()
	return ts, nil
}

func (tsc *tipSetCache) best() (*block.TipSet, error) {
	tsc.mu.RLock()
	best := tsc.cache[tsc.start]
	tsc.mu.RUnlock()
	if best == nil {
		return tsc.storage.ChainHead(context.TODO())
	}
	return best, nil
}

func normalModulo(n, m int) int {
	return ((n % m) + m) % m
}
