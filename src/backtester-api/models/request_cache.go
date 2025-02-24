package models

import (
	"fmt"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"

	pb "github.com/jiaming2012/slack-trading/src/playground"
)

type RequestCacheItem struct {
	mutex *sync.Mutex
	data  *pb.TickDelta
}

type RequestCache struct {
	cache *cache.Cache
}

func NewRequestCache() *RequestCache {
	return &RequestCache{
		cache: cache.New(5*time.Minute, 10*time.Minute),
	}
}

func (c *RequestCache) GetData(requestId string) chan *pb.TickDelta {
	ch := make(chan *pb.TickDelta)

	go func() {
		defer close(ch)
		log.Tracef("%v: waiting for cache", requestId)
		store, found := c.cache.Get(requestId)
		if !found {
			item := &RequestCacheItem{
				mutex: &sync.Mutex{},
				data:  nil,
			}

			item.mutex.Lock()
			c.cache.Set(requestId, item, cache.DefaultExpiration)
			ch <- nil
			log.Tracef("%v: cache not found, created new cache", requestId)
			return
		}

		item := store.(*RequestCacheItem)
		item.mutex.Lock()
		ch <- item.data
		log.Tracef("%v: cache found: setting data", requestId)
	}()

	return ch
}

func (c *RequestCache) StoreData(requestId string, data *pb.TickDelta) error {
	store, found := c.cache.Get(requestId)
	if !found {
		return fmt.Errorf("lock not found for request id: %s", requestId)
	}

	item := store.(*RequestCacheItem)
	item.data = data
	item.mutex.Unlock()
	log.Tracef("%v: data stored in cache", requestId)
	return nil
}
