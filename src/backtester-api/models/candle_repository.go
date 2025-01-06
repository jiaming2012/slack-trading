package models

import (
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
)

type CandleRepository struct {
	symbol                eventmodels.Instrument
	period                time.Duration
	interval              eventmodels.TradierInterval
	candlesWithIndicators []*eventmodels.AggregateBarWithIndicators
	baseCandles           []*eventmodels.PolygonAggregateBarV2
	indicators            []string
	position              int
	startingPosition      int
	newCandlesQueue       *eventmodels.FIFOQueue[*BacktesterCandle]
	mutex                 sync.Mutex
}

func (r *CandleRepository) GetSymbol() eventmodels.Instrument {
	return r.symbol
}

func (r *CandleRepository) GetPeriod() time.Duration {
	return r.period
}

func (r *CandleRepository) GetInterval() eventmodels.TradierInterval {
	return r.interval
}

func (r *CandleRepository) GetLastCandle() *eventmodels.AggregateBarWithIndicators {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if len(r.candlesWithIndicators) == 0 {
		return nil
	}

	return r.candlesWithIndicators[len(r.candlesWithIndicators)-1]
}

func (r *CandleRepository) AppendBars(bars []eventmodels.ICandle) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if len(bars) == 0 {
		return nil
	}

	for i, bar := range bars {
		if !r.candlesWithIndicators[len(r.candlesWithIndicators)-1].Timestamp.Before(bar.GetTimestamp()) {
			return fmt.Errorf("new bar[%d] is not after the last bar", i)
		}

		r.baseCandles = append(r.baseCandles, &eventmodels.PolygonAggregateBarV2{
			Timestamp: bar.GetTimestamp(),
			Open:      bar.GetOpen(),
			High:      bar.GetHigh(),
			Low:       bar.GetLow(),
			Close:     bar.GetClose(),
			Volume:    bar.GetVolume(),
		})
	}

	var err error
	previousIndex := len(r.candlesWithIndicators) - 1

	r.candlesWithIndicators, err = eventservices.AddIndicatorsToCandles(r.baseCandles, r.indicators)
	if err != nil {
		return fmt.Errorf("failed to aggregate bars with indicators: %v", err)
	}

	// send new bars to the queue
	if r.newCandlesQueue != nil {
		for i := previousIndex + 1; i < len(r.candlesWithIndicators); i++ {
			r.newCandlesQueue.Enqueue(&BacktesterCandle{
				Symbol: r.symbol,
				Period: r.period,
				Bar:    r.candlesWithIndicators[i],
			})
		}
	}

	return nil
}

func (r *CandleRepository) FetchCandles(startTime, endTime time.Time) ([]*eventmodels.AggregateBarWithIndicators, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var candles []*eventmodels.AggregateBarWithIndicators
	for _, candle := range r.candlesWithIndicators {
		if (candle.Timestamp.Equal(startTime) || candle.Timestamp.After(startTime)) && candle.Timestamp.Before(endTime) {
			candles = append(candles, candle)
		}
	}

	if len(candles) == 0 {
		log.Warnf("No candles found for %s between %s and %s", r.symbol, startTime, endTime)
	}

	return candles, nil
}

func (r *CandleRepository) FetchCandlesAtOrAfter(tstamp time.Time) (*eventmodels.AggregateBarWithIndicators, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, candle := range r.candlesWithIndicators {
		if candle.Timestamp.Equal(tstamp) || candle.Timestamp.After(tstamp) {
			return candle, nil
		}
	}

	log.Warnf("No candles found for %s at or after %s", r.symbol, tstamp)

	return nil, nil
}

func (r *CandleRepository) GetCurrentCandle() *eventmodels.AggregateBarWithIndicators {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.position >= len(r.candlesWithIndicators) {
		return nil
	}

	return r.candlesWithIndicators[r.position]
}

func (r *CandleRepository) Update(currentTime time.Time) (*eventmodels.AggregateBarWithIndicators, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.position >= len(r.candlesWithIndicators) {
		return nil, fmt.Errorf("no more candles")
	}

	var newCandle *eventmodels.AggregateBarWithIndicators
	for {
		if r.position >= len(r.candlesWithIndicators)-1 {
			break
		}

		nextCandleTimestamp := r.candlesWithIndicators[r.position+1].Timestamp
		if currentTime.Equal(nextCandleTimestamp) || currentTime.After(nextCandleTimestamp) {
			r.position++
			newCandle = r.GetCurrentCandle()
		} else if r.position == r.startingPosition {
			newCandle = r.GetCurrentCandle()
			break
		} else {
			break
		}
	}

	return newCandle, nil
}

func NewCandleRepository(symbol eventmodels.Instrument, period time.Duration, candles []*eventmodels.PolygonAggregateBarV2, indicators []string, newCandlesQueue *eventmodels.FIFOQueue[*BacktesterCandle], startingPosition int) (*CandleRepository, error) {
	var interval eventmodels.TradierInterval
	switch period {
	case time.Minute:
		interval = eventmodels.TradierInterval1Min
	case 5 * time.Minute:
		interval = eventmodels.TradierInterval5Min
	case 15 * time.Minute:
		interval = eventmodels.TradierInterval15Min
	default:
		log.Fatalf("Unsupported period: %s", period)
	}

	candlesWithIndicators, err := eventservices.AddIndicatorsToCandles(candles, indicators)
	if err != nil {
		return nil, fmt.Errorf("failed to add indicators to candles: %v", err)
	}

	return &CandleRepository{
		symbol:                symbol,
		period:                period,
		interval:              interval,
		candlesWithIndicators: candlesWithIndicators,
		baseCandles:           candles,
		position:              startingPosition,
		startingPosition:      startingPosition,
		indicators:            indicators,
		newCandlesQueue:       newCandlesQueue,
		mutex:                 sync.Mutex{},
	}, nil
}
