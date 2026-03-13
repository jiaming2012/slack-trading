package models

import (
	"sync"
	"time"
)

type DatafeedName string

const (
	CoinbaseDatafeed DatafeedName = "CoinbaseDatafeed"
	IBDatafeed       DatafeedName = "IBDatafeed"
	ManualDatafeed   DatafeedName = "ManualDatafeed"
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
	t.LastBid = tick.Bid
	t.LastOffer = tick.Ask
}

func (t *Datafeed) Tick() *Tick {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return &Tick{
		Timestamp: t.LastUpdate,
		Bid:       t.LastBid,
		Ask:       t.LastOffer,
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
