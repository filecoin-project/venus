package chain

import (
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/ipfs/go-datastore"
	"time"
)

type ICacheDs interface {
	Get(key datastore.Key) (value []byte, err error)
	Put(key datastore.Key, value []byte) error
	Delete(key datastore.Key) error
}

var _ ICacheDs = (*CacheDs)(nil)

type CacheDs struct {
	ds    repo.Datastore
	cache blockstoreutil.IBlockCache
}

func NewCacheDs(ds repo.Datastore, useLru bool) *CacheDs {
	var cache blockstoreutil.IBlockCache
	if useLru {
		cache = blockstoreutil.NewLruCache(10000)
	} else {
		cache = blockstoreutil.NewTimeCache(time.Hour*24, time.Hour*24)
	}
	return &CacheDs{ds: ds, cache: cache}
}

func (cacheDs CacheDs) Get(key datastore.Key) ([]byte, error) {
	if val, ok := cacheDs.cache.Get(key.String()); ok {
		return val.([]byte), nil
	}
	val, err := cacheDs.ds.Get(key)
	if err != nil {
		return nil, err
	}
	cacheDs.cache.Add(key.String(), val)
	return val, nil
}

func (cacheDs CacheDs) Put(key datastore.Key, value []byte) error {
	cacheDs.cache.Add(key.String(), value)
	return cacheDs.ds.Put(key, value)
}

func (cacheDs CacheDs) Delete(key datastore.Key) error {
	cacheDs.cache.Remove(key.String())
	return cacheDs.ds.Delete(key)
}
