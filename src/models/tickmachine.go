package models

import "time"

type TickMachine struct {
	LastUpdate time.Time
	LastBid    float64
	LastOffer  float64
}

func (t *TickMachine) Update(tick Tick) {
	t.LastUpdate = tick.Timestamp
	t.LastBid = tick.Bid
	t.LastOffer = tick.Ask
}

func (t *TickMachine) Query() *Tick {
	return &Tick{
		Timestamp: t.LastUpdate,
		Bid:       t.LastBid,
		Ask:       t.LastOffer,
	}
}

func NewTickMachine() *TickMachine {
	return &TickMachine{
		LastUpdate: time.Time{},
		LastBid:    0,
		LastOffer:  0,
	}
}
