package models

import "time"

type Candle struct {
	Timestamp   time.Time
	LastUpdated time.Time
	Open        float64
	High        float64
	Low         float64
	Close       float64
}

func (c *Candle) Update(price float64) {
	if price > c.High {
		c.High = price
	}

	if price < c.Low {
		c.Low = price
	}

	c.Close = price

	c.LastUpdated = time.Now()
}

func NewCandle(price float64) *Candle {
	timestamp := time.Now()
	return &Candle{
		Timestamp:   timestamp,
		LastUpdated: timestamp,
		Open:        price,
		High:        price,
		Low:         price,
		Close:       price,
	}
}
