package models

import "time"

type DatafeedName string

const (
	CoinbaseDatafeed DatafeedName = "CoinbaseDatafeed"
	ManualDatafeed   DatafeedName = "ManualDatafeed"
)

type Datafeed struct {
	Name       DatafeedName
	LastUpdate time.Time
	LastBid    float64
	LastOffer  float64
}

func (t *Datafeed) Update(tick Tick) {
	t.LastUpdate = tick.Timestamp
	t.LastBid = tick.Bid
	t.LastOffer = tick.Ask
}

func (t *Datafeed) Tick() *Tick {
	return &Tick{
		Timestamp: t.LastUpdate,
		Bid:       t.LastBid,
		Ask:       t.LastOffer,
	}
}

func (t *Datafeed) Bid() float64 {
	return t.LastBid
}

func (t *Datafeed) Offer() float64 {
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
