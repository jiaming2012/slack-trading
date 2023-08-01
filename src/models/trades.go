package models

import (
	"math"
	"time"
)

type Trades []*Trade
type Vwap float64
type Volume float64
type RealizedPL float64
type FloatingPL float64

func (trades *Trades) ToRows() [][]interface{} {
	results := make([][]interface{}, 0)

	for _, tr := range *trades {
		results = append(results, []interface{}{
			tr.Time.Format(time.RFC3339),
			tr.Symbol,
			tr.Volume,
			tr.RequestedPrice,
			tr.ExecutedPrice,
		})
	}

	return results
}

func (trades *Trades) Add(trade *Trade) {
	*trades = append(*trades, trade)
}

func (trades *Trades) Vwap() (Vwap, Volume, RealizedPL) {
	vwap := 0.0
	volume := 0.0
	realizedPL := 0.0

	for _, tr := range *trades {
		tradeWeight := tr.Volume / (tr.Volume + volume)

		if volume > 0 {
			if tr.Volume > 0 {
				vwap = ((1 - tradeWeight) * vwap) + (tradeWeight * tr.ExecutedPrice)
			} else {
				closeVolume := math.Min(volume, math.Abs(tr.Volume))
				realizedPL += (tr.ExecutedPrice - vwap) * closeVolume

				if math.Abs(tr.Volume) > volume {
					vwap = tr.ExecutedPrice
				}
			}
		} else if volume < 0 {
			if tr.Volume < 0 {
				vwap = ((1 - tradeWeight) * vwap) + (tradeWeight * tr.ExecutedPrice)
			} else {
				closeVolume := math.Min(math.Abs(volume), tr.Volume)
				realizedPL += (vwap - tr.ExecutedPrice) * closeVolume

				if tr.Volume > math.Abs(volume) {
					vwap = tr.ExecutedPrice
				}
			}
		} else {
			vwap = tr.ExecutedPrice
		}

		volume += tr.Volume
	}

	if volume == 0 {
		vwap = 0
	}

	return Vwap(vwap), Volume(volume), RealizedPL(realizedPL)
}

func (trades *Trades) PL(currentPrice float64) Profit {
	realizedPL := 0.0
	volume := 0.0
	placedTrades := make(Trades, 0)

	for _, tr := range *trades {
		vwap, _, _ := placedTrades.Vwap()
		if volume > 0 {
			if tr.Side() == TradeTypeSell {
				realizedPL += math.Abs(tr.Volume) * (tr.ExecutedPrice - float64(vwap))
			}
		} else if volume < 0 {
			if tr.Side() == TradeTypeBuy {
				realizedPL += math.Abs(tr.Volume) * (float64(vwap) - tr.ExecutedPrice)
			}
		} else {
		}

		placedTrades.Add(tr)
		volume += tr.Volume
	}

	floatingPL := 0.0
	vwap, _, _ := placedTrades.Vwap()
	if volume > 0 {
		floatingPL = (currentPrice - float64(vwap)) * volume
	} else if volume < 0 {
		floatingPL = (float64(vwap) - currentPrice) * math.Abs(volume)
	} else {
	}

	return Profit{
		Floating: floatingPL,
		Realized: realizedPL,
		Volume:   Volume(volume),
	}
}
