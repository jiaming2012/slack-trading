package models

import (
	"fmt"
	"math"
	"time"
)

type Candle struct {
	Timestamp   time.Time
	LastUpdated time.Time
	Open        float64
	High        float64
	Low         float64
	Close       float64
}

type Candles struct {
	Period int
	Data   []Candle
}

func (cs *Candles) ToRows() [][]interface{} {
	results := make([][]interface{}, 0)

	for _, c := range cs.Data {
		results = append(results, []interface{}{
			c.Timestamp.Format(time.RFC3339),
			c.LastUpdated.Format(time.RFC3339),
			c.Open,
			c.High,
			c.Low,
			c.Close,
		})
	}

	return results
}

func (cs *Candles) Add(candle *Candle) {
	cs.Data = append(cs.Data, *candle)
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

func (cs *Candles) ConvertTo(period int) (*Candles, error) {
	if math.Mod(float64(period), float64(cs.Period)) != 0 {
		return nil, fmt.Errorf("cannot divide %v into %v", period, cs.Period)
	}

	if period < cs.Period {
		return nil, fmt.Errorf("period of %v is less than base period of %v", period, cs.Period)
	}

	candlesSize := (len(cs.Data) * cs.Period) / period
	if math.Mod(float64(len(cs.Data)*cs.Period), float64(period)) != 0 {
		candlesSize += 1
	}

	result := Candles{
		Period: period,
		Data:   make([]Candle, int(candlesSize)),
	}

	var insert *Candle
	k := 0
	for i, c := range cs.Data {
		if math.Mod(float64(i*cs.Period), float64(period)) == 0 {
			if insert != nil {
				result.Data[k] = *insert
				k += 1
			}

			insert = NewCandle(c.Open)
		}

		if c.High > insert.High {
			insert.High = c.High
		}

		if c.Low < insert.Low {
			insert.Low = c.Low
		}

		insert.Close = c.Close
	}

	// insert final candle
	result.Data[k] = *insert

	return &result, nil
}
