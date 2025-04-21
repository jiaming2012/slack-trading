package models

import (
	"github.com/jinzhu/copier"
	logger "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type PositionsCache struct {
	cache map[eventmodels.Instrument]*Position
}

func (o *PositionsCache) Update(symbol eventmodels.Instrument, pl float64, currentPrice float64) {
	if o.cache == nil {
		logger.Warnf("PositionsCache is nil: ignoring update ...")
		return
	}

	if _, ok := o.cache[symbol]; !ok {
		logger.Warnf("PositionsCache: symbol %s not found: ignoring update ...", symbol)
		return
	}
	// Update the position with the new P&L and current price
	o.cache[symbol].PL = pl
	o.cache[symbol].CurrentPrice = currentPrice
}

func (o *PositionsCache) Set(symbol eventmodels.Instrument, position *Position) {
	if o.cache == nil {
		o.cache = make(map[eventmodels.Instrument]*Position)
	}

	if _, ok := o.cache[symbol]; !ok {
		o.cache[symbol] = &Position{}
	}

	// Append the order to the slice for the given symbol
	o.cache[symbol] = position
}

func (o *PositionsCache) Len() int {
	return len(o.cache)
}

func (o *PositionsCache) Iter() map[eventmodels.Instrument]*Position {
	if o.cache == nil {
		return nil
	}
	return o.cache
}

func (o *PositionsCache) Delete(symbol eventmodels.Instrument) {
	if o.cache == nil {
		return
	}

	if _, ok := o.cache[symbol]; !ok {
		return
	}

	// Remove the order at the specified index
	delete(o.cache, symbol)
}

func (o *PositionsCache) Add(symbol eventmodels.Instrument, trade *TradeRecord) {
	if o.cache == nil {
		o.cache = make(map[eventmodels.Instrument]*Position)
	}

	if _, ok := o.cache[symbol]; !ok {
		o.cache[symbol] = &Position{}
	}

	// Append the order to the slice for the given symbol
	o.cache[symbol].Quantity += trade.Quantity
}

func (o *PositionsCache) Get(symbol eventmodels.Instrument) *Position {
	if o.cache == nil {
		return &Position{}
	}

	pos, found := o.cache[symbol]
	if !found {
		return &Position{}
	}

	return pos
}

func (o *PositionsCache) SetCache(cache map[eventmodels.Instrument]*Position) {
	o.cache = cache
}

func (o *PositionsCache) Commit(obj *PositionsCache) {
	o.cache = obj.cache
}

func (o *PositionsCache) Copy() *PositionsCache {
	copy := &PositionsCache{}
	copier.Copy(&copy.cache, o.cache)
	return copy
}

func NewPositionsCache() *PositionsCache {
	return &PositionsCache{
		cache: make(map[eventmodels.Instrument]*Position),
	}
}
