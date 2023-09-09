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

func (trades *Trades) Copy() *Trades {
	result := &Trades{}

	for _, tr := range *trades {
		result.Add(&Trade{
			ID:              tr.ID,
			Type:            tr.Type,
			Symbol:          tr.Symbol,
			Timestamp:       tr.Timestamp,
			RequestedVolume: tr.RequestedVolume,
			ExecutedVolume:  tr.ExecutedVolume,
			ExecutedPrice:   tr.ExecutedPrice,
			RequestedPrice:  tr.RequestedPrice,
			StopLoss:        tr.StopLoss,
			Offsets:         tr.Offsets,
		})
	}

	return result
}

func (trades *Trades) ToRows() [][]interface{} {
	results := make([][]interface{}, 0)

	for _, tr := range *trades {
		results = append(results, []interface{}{
			tr.Timestamp.Format(time.RFC3339),
			tr.Symbol,
			tr.RequestedVolume,
			tr.RequestedPrice,
			tr.ExecutedPrice,
		})
	}

	return results
}

func (trades *Trades) Add(trade *Trade) {
	*trades = append(*trades, trade)
}

func (trades *Trades) BulkAdd(newTrades *Trades) {
	for _, t := range *newTrades {
		trades.Add(t)
	}
}

func (trades *Trades) Vwap() (Vwap, Volume, RealizedPL) {
	vwap := 0.0
	volume := 0.0
	realizedPL := 0.0

	for _, tr := range *trades {
		tradeWeight := tr.RequestedVolume / (tr.RequestedVolume + volume)

		if volume > 0 {
			if tr.RequestedVolume > 0 {
				vwap = ((1 - tradeWeight) * vwap) + (tradeWeight * tr.ExecutedPrice)
			} else {
				closeVolume := math.Min(volume, math.Abs(tr.RequestedVolume))
				realizedPL += (tr.ExecutedPrice - vwap) * closeVolume

				if math.Abs(tr.RequestedVolume) > volume {
					vwap = tr.ExecutedPrice
				}
			}
		} else if volume < 0 {
			if tr.RequestedVolume < 0 {
				vwap = ((1 - tradeWeight) * vwap) + (tradeWeight * tr.ExecutedPrice)
			} else {
				closeVolume := math.Min(math.Abs(volume), tr.RequestedVolume)
				realizedPL += (vwap - tr.ExecutedPrice) * closeVolume

				if tr.RequestedVolume > math.Abs(volume) {
					vwap = tr.ExecutedPrice
				}
			}
		} else {
			vwap = tr.ExecutedPrice
		}

		volume += tr.RequestedVolume
	}

	if volume == 0 {
		vwap = 0
	}

	return Vwap(vwap), Volume(volume), RealizedPL(realizedPL)
}

func (trades *Trades) PL(tick Tick) Profit {
	realizedPL := 0.0
	volume := 0.0
	placedTrades := make(Trades, 0)

	for _, tr := range *trades {
		vwap, _, _ := placedTrades.Vwap()
		if volume > 0 {
			if tr.Side() == TradeTypeSell {
				realizedPL += math.Abs(tr.RequestedVolume) * (tr.ExecutedPrice - float64(vwap))
			}
		} else if volume < 0 {
			if tr.Side() == TradeTypeBuy {
				realizedPL += math.Abs(tr.RequestedVolume) * (float64(vwap) - tr.ExecutedPrice)
			}
		} else {
		}

		placedTrades.Add(tr)
		volume += tr.RequestedVolume
	}

	floatingPL := 0.0
	vwap, _, _ := placedTrades.Vwap()
	if volume > 0 {
		floatingPL = (tick.Bid - float64(vwap)) * volume
	} else if volume < 0 {
		floatingPL = (float64(vwap) - tick.Ask) * math.Abs(volume)
	} else {
	}

	return Profit{
		Floating: floatingPL,
		Realized: realizedPL,
		Volume:   Volume(volume),
	}
}
