package eventmodels

import (
	"sync"
	"time"
)

type Datafeed struct {
	Name       DatafeedName `json:"name"`
	LastUpdate time.Time    `json:"lastUpdate"`
	LastBid    float64      `json:"lastBid"`
	LastOffer  float64      `json:"lastOffer"`
	mu         sync.RWMutex
}

func (t *Datafeed) Update(tick Tick) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.LastUpdate = tick.Timestamp
	t.LastBid = tick.Price

	if t.LastOffer > 0 {
		panic("Off is not yet implemented")
	}
}

func (t *Datafeed) Tick() *Tick {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return &Tick{
		Timestamp: t.LastUpdate,
		Price:     t.LastBid,
	}
}

func (t *Datafeed) Bid() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.LastBid
}

func (t *Datafeed) Offer() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.LastOffer
}

func NewDatafeed(name DatafeedName) *Datafeed {
	return &Datafeed{
		Name:       name,
		LastUpdate: time.Time{},
		LastBid:    0,
		LastOffer:  0,
	}
}
