package models

import (
	"math"
	"time"
)

type Trades []Trade

func (trades *Trades) ToRows() [][]interface{} {
	results := make([][]interface{}, 0)

	for _, tr := range *trades {
		results = append(results, []interface{}{
			tr.Time.Format(time.RFC3339),
			tr.Symbol,
			tr.Volume,
			tr.Price,
		})
	}

	return results
}

func (trades *Trades) Add(trade *Trade) {
	*trades = append(*trades, *trade)
}

func (trades *Trades) Vwap() float64 {
	vwap := 0.0
	volume := 0.0

	for _, tr := range *trades {
		tradeWeight := tr.Volume / (tr.Volume + volume)

		if volume > 0 {
			if tr.Volume > 0 {
				vwap = ((1 - tradeWeight) * vwap) + (tradeWeight * tr.Price)
			} else {
				if math.Abs(tr.Volume) > volume {
					vwap = tr.Price
				}
			}
		} else if volume < 0 {
			if tr.Volume < 0 {
				vwap = ((1 - tradeWeight) * vwap) + (tradeWeight * tr.Price)
			} else {
				if tr.Volume > math.Abs(volume) {
					vwap = tr.Price
				}
			}
		} else {
			vwap = tr.Price
		}

		volume += tr.Volume
	}

	return vwap
}

func (trades *Trades) PL(currentPrice float64) Profit {
	realizedPL := 0.0
	volume := 0.0
	placedTrades := make(Trades, 0)

	for _, tr := range *trades {
		vwap := placedTrades.Vwap()

		if volume > 0 {
			if tr.Side() == TradeTypeSell {
				realizedPL += math.Abs(tr.Volume) * (tr.Price - vwap)
			}
		} else if volume < 0 {
			if tr.Side() == TradeTypeBuy {
				realizedPL += math.Abs(tr.Volume) * (vwap - tr.Price)
			}
		} else {
		}

		placedTrades.Add(&tr)
		volume += tr.Volume
	}

	floatingPL := 0.0
	vwap := placedTrades.Vwap()
	if volume > 0 {
		floatingPL = (currentPrice - vwap) * volume
	} else if volume < 0 {
		floatingPL = (vwap - currentPrice) * math.Abs(volume)
	} else {
	}

	return Profit{
		Floating: floatingPL,
		Realized: realizedPL,
	}
}
