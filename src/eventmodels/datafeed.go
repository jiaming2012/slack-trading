package eventmodels

import (
	"sync"
	"time"
)

type Datafeed struct {
	Name       DatafeedName `json:"name"`
	LastUpdate time.Time    `json:"lastUpdate"`
	LastTick   float64      `json:"lastTick"`
	mu         sync.RWMutex
}

func (t *Datafeed) Update(tick Tick) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.LastUpdate = tick.Timestamp
	t.LastTick = tick.Price
}

func (t *Datafeed) Tick() *Tick {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return &Tick{
		Timestamp: t.LastUpdate,
		Price:     t.LastTick,
	}
}

func NewDatafeed(name DatafeedName) *Datafeed {
	return &Datafeed{
		Name:       name,
		LastUpdate: time.Time{},
		LastTick:   0,
	}
}
