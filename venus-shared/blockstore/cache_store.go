package blockstoreutil

import (
	"time"

	"github.com/bluele/gcache"
	tcache "github.com/patrickmn/go-cache"
)

type IBlockCache interface {
	Get(key string) (value interface{}, ok bool)
	Remove(key string)
	Add(key string, value interface{})
	AddWithExpire(key string, value interface{}, dur time.Duration)
}

var _ IBlockCache = (*TimeCache)(nil)

type TimeCache struct {
	cache *tcache.Cache
}

func NewTimeCache(expireTime, cleanTime time.Duration) *TimeCache {
	tCache := tcache.New(expireTime, cleanTime)
	return &TimeCache{cache: tCache}
}

func (timeCache TimeCache) Get(key string) (interface{}, bool) {
	return timeCache.cache.Get(key)
}

func (timeCache TimeCache) Remove(key string) {
	timeCache.cache.Delete(key)
}

func (timeCache TimeCache) Add(key string, value interface{}) {
	timeCache.cache.Set(key, value, 0)
}

func (timeCache TimeCache) AddWithExpire(key string, value interface{}, dur time.Duration) {
	timeCache.cache.Set(key, value, dur)
}

var _ IBlockCache = (*LruCache)(nil)

type LruCache struct {
	cache gcache.Cache
}

func NewLruCache(size int) *LruCache {
	cache := gcache.New(size).LRU().Build()
	go printRate(cache)
	return &LruCache{cache: cache}
}

func (l LruCache) Get(key string) (interface{}, bool) {
	val, err := l.cache.Get(key)
	return val, err == nil
}

func (l LruCache) Remove(key string) {
	l.cache.Remove(key)
}

func (l LruCache) Add(key string, value interface{}) {
	_ = l.cache.Set(key, value)
}

func (l LruCache) AddWithExpire(key string, value interface{}, dur time.Duration) {
	_ = l.cache.SetWithExpire(key, value, dur)
}

func printRate(cache gcache.Cache) {
	tm := time.NewTicker(time.Minute)
	for range tm.C {
		log.Infof("lru database cache hitrate:%f", cache.HitRate())
	}
}
