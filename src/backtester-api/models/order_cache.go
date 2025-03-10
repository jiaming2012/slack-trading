package models

import (
	"sync"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type OrderCache struct {
	container map[uint]ExecutionFillRequest
	mutex     *sync.Mutex
}

func (c *OrderCache) Add(order *eventmodels.TradierOrder, entry ExecutionFillRequest) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.container[order.ID] = entry
}

func (c *OrderCache) Get(order *eventmodels.TradierOrder) (ExecutionFillRequest, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry, ok := c.container[order.ID]
	return entry, ok
}

func (c *OrderCache) GetMap() (container map[uint]ExecutionFillRequest, unlockFn func()) {
	c.mutex.Lock()
	container = c.container

	unlockFn = func() {
		c.mutex.Unlock()
	}

	return
}

func (c *OrderCache) Remove(orderID uint, getMutex bool) {
	if getMutex {
		c.mutex.Lock()
		defer c.mutex.Unlock()
	}

	delete(c.container, orderID)
}

func NewOrderCache() *OrderCache {
	return &OrderCache{
		container: make(map[uint]ExecutionFillRequest),
		mutex:     &sync.Mutex{},
	}
}
