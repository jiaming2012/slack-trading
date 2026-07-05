package models

import (
	"sync"

	"github.com/jinzhu/copier"
)

type OpenOrdersCache struct {
	cache map[string][]*OrderRecord
	mu    sync.RWMutex
}

func (o *OpenOrdersCache) Len() int {
	o.mu.RLock()
	defer o.mu.RUnlock()

	size := 0

	for _, orders := range o.cache {
		totalQty := 0.0
		for _, order := range orders {
			totalQty += order.GetFilledVolume()
		}

		if totalQty != 0 {
			size += 1
		}
	}

	return size
}

func (o *OpenOrdersCache) Iter() (out map[string][]*OrderRecord, done func()) {
	if o.cache == nil {
		return nil, nil
	}

	o.mu.RLock()

	done = func() {
		o.mu.RUnlock()
	}

	out = o.cache

	return out, done
}

func (o *OpenOrdersCache) Delete(ticker string, index int) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.cache == nil {
		return
	}

	if _, ok := o.cache[ticker]; !ok {
		return
	}

	if index < 0 || index >= len(o.cache[ticker]) {
		return
	}

	// Remove the order at the specified index
	o.cache[ticker] = append(o.cache[ticker][:index], o.cache[ticker][index+1:]...)
}

func (o *OpenOrdersCache) Add(order *OrderRecord) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.cache == nil {
		o.cache = make(map[string][]*OrderRecord)
	}

	if _, ok := o.cache[order.instrument.GetTicker()]; !ok {
		o.cache[order.instrument.GetTicker()] = []*OrderRecord{}
	}

	// Append the order to the slice for the given symbol
	o.cache[order.instrument.GetTicker()] = append(o.cache[order.instrument.GetTicker()], order)
}

func (o *OpenOrdersCache) Get(ticker string) []*OrderRecord {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.cache == nil {
		o.cache = make(map[string][]*OrderRecord)
	}

	result, found := o.cache[ticker]
	if !found {
		return []*OrderRecord{}
	}

	return result
}

func (o *OpenOrdersCache) Commit(obj *OpenOrdersCache) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.cache = obj.cache
}

func (o *OpenOrdersCache) Copy() *OpenOrdersCache {
	o.mu.RLock()
	defer o.mu.RUnlock()

	copy := &OpenOrdersCache{}
	copier.Copy(&copy.cache, o.cache)
	return copy
}

func NewOpenOrdersCache() *OpenOrdersCache {
	return &OpenOrdersCache{
		cache: make(map[string][]*OrderRecord),
	}
}
