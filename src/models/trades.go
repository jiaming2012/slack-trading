package models

import (
	"fmt"
	"math"
	"sync"
	"time"
)

type Trades []*Trade
type Vwap float64
type Volume float64
type RealizedPL float64
type FloatingPL float64

func (vwap Vwap) Validate() error {
	if math.IsNaN(float64(vwap)) {
		return fmt.Errorf("vwap.Validate: NaN is not a valid value")
	}

	if math.IsInf(float64(vwap), 0) {
		return fmt.Errorf("vwap.Validate: +/- Inf is not a valid value")
	}

	return nil
}

func (volume Volume) Validate() error {
	if math.IsNaN(float64(volume)) {
		return fmt.Errorf("vwap.volume: NaN is not a valid value")
	}

	if math.IsInf(float64(volume), 0) {
		return fmt.Errorf("vwap.volume: +/- Inf is not a valid value")
	}

	return nil
}

func (realizedPL RealizedPL) Validate() error {
	if math.IsNaN(float64(realizedPL)) {
		return fmt.Errorf("realizedPL.Validate: NaN is not a valid value")
	}

	if math.IsInf(float64(realizedPL), 0) {
		return fmt.Errorf("realizedPL.Validate: +/- Inf is not a valid value")
	}

	return nil
}

func (floatingPL FloatingPL) Validate() error {
	if math.IsNaN(float64(floatingPL)) {
		return fmt.Errorf("vwap.Validate: NaN is not a valid value")
	}

	if math.IsInf(float64(floatingPL), 0) {
		return fmt.Errorf("vwap.Validate: +/- Inf is not a valid value")
	}

	return nil
}

type TradeGroup struct {
	Trades Trades
	mutex  sync.Mutex
}

func (trades *Trades) OpenTrades() *Trades {
	tradeToClosedVolumeMap := make(map[*Trade]float64)
	openTrades := &Trades{}

	for _, tr := range *trades {
		if tr.Type == TradeTypeClose {
			if math.Abs(tr.ExecutedVolume) < math.SmallestNonzeroFloat64 || tr.ExecutedVolume == math.NaN() {
				continue
			}

			closeVol := math.Abs(tr.ExecutedVolume)
			usedVol := 0.0
			for _, off := range tr.Offsets {
				usedVol += math.Min(closeVol, math.Abs(off.ExecutedVolume))
				tradeToClosedVolumeMap[off] += usedVol
				closeVol -= usedVol

				if closeVol <= 0 {
					break
				}
			}
		}
	}

	for _, tr := range *trades {
		if math.Abs(tr.ExecutedVolume) < math.SmallestNonzeroFloat64 || tr.ExecutedVolume == math.NaN() {
			continue
		}

		if tr.Type == TradeTypeBuy || tr.Type == TradeTypeSell {
			offsettingVolume := tradeToClosedVolumeMap[tr]
			if math.Abs(tr.ExecutedVolume) > offsettingVolume {
				openTrades.Add(tr)
			}
		}
	}

	return openTrades
}

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

func (trades *Trades) MaxRisk(stopLoss float64) (float64, RealizedPL) {
	vwap, vol, realizedPL := trades.GetTradeStatsItems()

	var maxRisk float64
	if vol != 0 {
		maxRisk = math.Abs(stopLoss-float64(vwap)) * math.Abs(float64(vol))
	} else {
		maxRisk = 0
	}

	return maxRisk, realizedPL
}

func (trades *Trades) GetTradeStatsItems() (Vwap, Volume, RealizedPL) {
	vwap := 0.0
	volume := 0.0
	realizedPL := 0.0

	for _, tr := range *trades {
		if math.Abs(tr.ExecutedVolume) < math.SmallestNonzeroFloat64 || tr.ExecutedVolume == math.NaN() {
			continue
		}

		tradeWeight := tr.ExecutedVolume / (tr.ExecutedVolume + volume)

		if volume > 0 {
			if tr.ExecutedVolume > 0 {
				vwap = ((1 - tradeWeight) * vwap) + (tradeWeight * tr.ExecutedPrice)
			} else {
				closeVolume := math.Min(volume, math.Abs(tr.ExecutedVolume))
				realizedPL += (tr.ExecutedPrice - vwap) * closeVolume

				if math.Abs(tr.ExecutedVolume) > volume {
					vwap = tr.ExecutedPrice
				}
			}
		} else if volume < 0 {
			if tr.ExecutedVolume < 0 {
				vwap = ((1 - tradeWeight) * vwap) + (tradeWeight * tr.ExecutedPrice)
			} else {
				closeVolume := math.Min(math.Abs(volume), tr.ExecutedVolume)
				realizedPL += (vwap - tr.ExecutedPrice) * closeVolume

				if tr.ExecutedVolume > math.Abs(volume) {
					vwap = tr.ExecutedPrice
				}
			}
		} else {
			vwap = tr.ExecutedPrice
		}

		volume += tr.ExecutedVolume
	}

	if volume == 0 {
		vwap = 0
	}

	return Vwap(vwap), Volume(volume), RealizedPL(realizedPL)
}

func (trades *Trades) GetTradeStats(tick Tick) (TradeStats, error) {
	realizedPL := 0.0
	volume := 0.0
	placedTrades := make(Trades, 0)

	for _, tr := range *trades {
		if math.Abs(tr.ExecutedVolume) < math.SmallestNonzeroFloat64 || tr.ExecutedVolume == math.NaN() {
			continue
		}

		vwap, _, _ := placedTrades.GetTradeStatsItems()
		if err := vwap.Validate(); err != nil {
			return TradeStats{}, fmt.Errorf("Trades.GetTradeStats vwap validation failed: %w", err)
		}

		if volume > 0 {
			if tr.Side() == TradeTypeSell {
				realizedPL += math.Abs(tr.ExecutedVolume) * (tr.ExecutedPrice - float64(vwap))
			}
		} else if volume < 0 {
			if tr.Side() == TradeTypeBuy {
				realizedPL += math.Abs(tr.ExecutedVolume) * (float64(vwap) - tr.ExecutedPrice)
			}
		} else {
		}

		placedTrades.Add(tr)
		volume += tr.ExecutedVolume
	}

	floatingPL := 0.0
	vwap, _, _ := placedTrades.GetTradeStatsItems()
	if err := vwap.Validate(); err != nil {
		return TradeStats{}, fmt.Errorf("Trades.GetTradeStats vwap validation to calculate floatingPL failed: %w", err)
	}
	if volume > 0 {
		floatingPL = (tick.Bid - float64(vwap)) * volume
	} else if volume < 0 {
		floatingPL = (float64(vwap) - tick.Ask) * math.Abs(volume)
	} else {
	}

	_floatingPL := FloatingPL(floatingPL)
	if err := _floatingPL.Validate(); err != nil {
		return TradeStats{}, fmt.Errorf("Trades.GetTradeStats: failed to validate floatingPL: %w", err)
	}

	_realizedPL := RealizedPL(realizedPL)
	if err := _realizedPL.Validate(); err != nil {
		return TradeStats{}, fmt.Errorf("Trades.GetTradeStats: failed to validate realizedPL: %w", err)
	}

	_volume := Volume(volume)

	return TradeStats{
		Floating: floatingPL,
		Realized: realizedPL,
		Volume:   _volume,
		Vwap:     vwap,
	}, nil
}
