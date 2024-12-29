package models

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type BacktesterCandleRepository struct {
	symbol   eventmodels.Instrument
	period   time.Duration
	candles  []*eventmodels.PolygonAggregateBarV2
	position int
}

func (r *BacktesterCandleRepository) GetSymbol() eventmodels.Instrument {
	return r.symbol
}

func (r *BacktesterCandleRepository) GetPeriod() time.Duration {
	return r.period
}

func (r *BacktesterCandleRepository) FetchCandles(startTime, endTime time.Time) ([]*eventmodels.PolygonAggregateBarV2, error) {
	var candles []*eventmodels.PolygonAggregateBarV2
	for _, candle := range r.candles {
		if (candle.Timestamp.Equal(startTime) || candle.Timestamp.After(startTime)) && candle.Timestamp.Before(endTime) {
			candles = append(candles, candle)
		}
	}

	if len(candles) == 0 {
		log.Warnf("No candles found for %s between %s and %s", r.symbol, startTime, endTime)
	}

	return candles, nil
}

func (r *BacktesterCandleRepository) FetchCandlesAtOrAfter(tstamp time.Time) (*eventmodels.PolygonAggregateBarV2, error) {
	for _, candle := range r.candles {
		if candle.Timestamp.Equal(tstamp) || candle.Timestamp.After(tstamp) {
			return candle, nil
		}
	}

	log.Warnf("No candles found for %s at or after %s", r.symbol, tstamp)

	return nil, nil
}

func (r *BacktesterCandleRepository) GetCurrentCandle() *eventmodels.PolygonAggregateBarV2 {
	if r.position >= len(r.candles) {
		return nil
	}

	return r.candles[r.position]
}

func (r *BacktesterCandleRepository) Update(currentTime time.Time) (*eventmodels.PolygonAggregateBarV2, error) {
	if r.position >= len(r.candles) {
		return nil, fmt.Errorf("no more candles")
	}

	var newCandle *eventmodels.PolygonAggregateBarV2
	for {
		if r.position >= len(r.candles)-1 {
			break
		}

		nextCandleTimestamp := r.candles[r.position+1].Timestamp
		if currentTime.Equal(nextCandleTimestamp) || currentTime.After(nextCandleTimestamp) {
			r.position++
			newCandle = r.GetCurrentCandle()
		} else {
			break
		}
	}

	return newCandle, nil
}

type Indicators map[string][]float64

func NewBacktesterCandleRepository(symbol eventmodels.Instrument, period time.Duration, candles []*eventmodels.PolygonAggregateBarV2, indicatorNames ...string) *BacktesterCandleRepository {
	
	
	
	return &BacktesterCandleRepository{
		symbol:   symbol,
		period:   period,
		candles:  candles,
		position: 0,
	}
}
