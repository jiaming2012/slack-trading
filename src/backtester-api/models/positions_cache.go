package models

import (
	"log"
	"time"

	"github.com/jinzhu/copier"
	logger "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type PositionsCache struct {
	cache       map[string]*Position
	instruments map[string]eventmodels.Instrument
}

func (o *PositionsCache) Update(symbol eventmodels.Instrument, pl float64, currentPrice float64, timestamp time.Time) {
	if o.cache == nil {
		logger.Warnf("PositionsCache is nil: ignoring update ...")
		return
	}

	ticker := symbol.GetTicker()
	if _, ok := o.cache[ticker]; !ok {
		logger.Warnf("PositionsCache: symbol %s not found: ignoring update ...", symbol)
		return
	}

	// Ensure instruments is in sync
	o.instruments[ticker] = symbol

	// Update the position with the new P&L and current price
	o.cache[ticker].PL = pl
	o.cache[ticker].CurrentPrice = currentPrice
	o.cache[ticker].Timestamp = timestamp.Format(time.RFC3339)
}

func (o *PositionsCache) Set(symbol eventmodels.Instrument, position *Position) {
	if o.cache == nil {
		o.cache = make(map[string]*Position)
	}

	if o.instruments == nil {
		o.instruments = make(map[string]eventmodels.Instrument)
	}

	ticker := symbol.GetTicker()
	if _, ok := o.cache[ticker]; !ok {
		o.cache[ticker] = &Position{}
		o.instruments[ticker] = symbol
	}

	// Append the order to the slice for the given symbol
	o.cache[ticker] = position
}

func (o *PositionsCache) Len() int {
	return len(o.cache)
}

func (o *PositionsCache) Iter() map[eventmodels.Instrument]*Position {
	if o.cache == nil {
		return nil
	}

	result := make(map[eventmodels.Instrument]*Position)
	for k, v := range o.cache {
		instrument, found := o.instruments[k]
		if !found {
			log.Fatalf("PositionsCache: instrument for ticker %s not found", k)
		}

		result[instrument] = v
	}
	return result
}

func (o *PositionsCache) Delete(symbol eventmodels.Instrument) {
	if o.cache == nil {
		return
	}

	ticker := symbol.GetTicker()
	if _, ok := o.cache[ticker]; !ok {
		return
	}

	// Remove the order at the specified index
	delete(o.cache, ticker)
	delete(o.instruments, ticker)
}

func (o *PositionsCache) Add(symbol eventmodels.Instrument, trade *TradeRecord) {
	if o.cache == nil {
		o.cache = make(map[string]*Position)
	}

	if o.instruments == nil {
		o.instruments = make(map[string]eventmodels.Instrument)
	}

	ticker := symbol.GetTicker()
	if _, ok := o.cache[ticker]; !ok {
		o.cache[ticker] = &Position{}
		o.instruments[ticker] = symbol
	}

	// Update the cost basis
	if o.cache[ticker].Quantity > 0 {
		if trade.Quantity > 0 {
			o.cache[ticker].CostBasis = (o.cache[ticker].CostBasis*o.cache[ticker].Quantity + trade.Price*trade.Quantity) / (o.cache[ticker].Quantity + trade.Quantity)
		}
	} else if o.cache[ticker].Quantity < 0 {
		if trade.Quantity < 0 {
			o.cache[ticker].CostBasis = (o.cache[ticker].CostBasis*o.cache[ticker].Quantity + trade.Price*trade.Quantity) / (o.cache[ticker].Quantity + trade.Quantity)
		}
	} else {
		o.cache[ticker].CostBasis = trade.Price
	}

	// Update the maintenance margin
	o.cache[ticker].MaintenanceMargin = calculateMaintenanceRequirement(trade.Quantity, trade.Price)

	// Append the order to the slice for the given symbol
	o.cache[ticker].Quantity += trade.Quantity
}

func (o *PositionsCache) Get(ticker string) *Position {
	if o.cache == nil {
		return &Position{}
	}

	pos, found := o.cache[ticker]
	if !found {
		return &Position{}
	}

	return pos
}

func (o *PositionsCache) Exists(symbol eventmodels.Instrument) bool {
	if o.cache == nil {
		return false
	}

	_, found := o.cache[symbol.GetTicker()]
	return found
}

func (o *PositionsCache) SetCache(cache map[eventmodels.Instrument]*Position) {
	internal := make(map[string]*Position)
	instruments := make(map[string]eventmodels.Instrument)

	for k, v := range cache {
		ticker := k.GetTicker()

		instruments[ticker] = k
		internal[ticker] = v
	}

	o.cache = internal
	o.instruments = instruments
}

func (o *PositionsCache) Commit(obj *PositionsCache) {
	o.cache = obj.cache
	o.instruments = obj.instruments
}

func (o *PositionsCache) Copy() *PositionsCache {
	copy := &PositionsCache{}
	copier.Copy(&copy.cache, o.cache)
	copier.Copy(&copy.instruments, o.instruments)
	return copy
}

func NewPositionCache() *PositionsCache {
	return &PositionsCache{
		cache:       make(map[string]*Position),
		instruments: make(map[string]eventmodels.Instrument),
	}
}
