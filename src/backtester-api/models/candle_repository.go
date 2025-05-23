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
	periodStr             string
	fetchInterval         eventmodels.TradierInterval
	polygonTimespan       eventmodels.PolygonTimespan
	candlesWithIndicators []*eventmodels.AggregateBarWithIndicators
	baseCandles           []*eventmodels.PolygonAggregateBarV2
	indicators            []string
	position              int
	startingPosition      *int
	newCandlesQueue       *eventmodels.FIFOQueue[*BacktesterCandle]
	isInitialTick         bool
	historyInDays         uint32
	nextUpdateAt          *time.Time
	source                eventmodels.CandleRepositorySource
	mutex                 *sync.Mutex
}

func (r *CandleRepository) SetNextUpdateAt(lastTstamp time.Time) time.Time {
	updateAt := lastTstamp.Add(2 * r.period)
	r.nextUpdateAt = &updateAt
	return *r.nextUpdateAt
}

func (r *CandleRepository) GetNextUpdateAt() *time.Time {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.nextUpdateAt
}

func (r *CandleRepository) ToDTO() CandleRepositoryDTO {
	return CandleRepositoryDTO{
		Symbol:                   r.symbol.GetTicker(),
		Duration:                 r.period,
		FetchInterval:            string(r.fetchInterval),
		PolygonTimespanMultipler: r.polygonTimespan.Multiplier,
		PolygonTimespanUnit:      string(r.polygonTimespan.Unit),
		Indicators:               r.indicators,
		Position:                 r.position,
		StartingPosition:         r.startingPosition,
		HistoryInDays:            r.historyInDays,
		SourceType:               r.source.Type,
		IsInitialTick:            r.isInitialTick,
	}
}

func (r *CandleRepository) GetSymbol() eventmodels.Instrument {
	return r.symbol
}

func (r *CandleRepository) GetIndicators() []string {
	return r.indicators
}

func (r *CandleRepository) GetPeriod() time.Duration {
	return r.period
}

func (r *CandleRepository) GetPeriodStr() string {
	return r.periodStr
}

func (r *CandleRepository) GetPolygonTimespan() eventmodels.PolygonTimespan {
	return r.polygonTimespan
}

func (r *CandleRepository) GetFetchInterval() eventmodels.TradierInterval {
	return r.fetchInterval
}

func (r *CandleRepository) GetHistoryInDays() uint32 {
	return r.historyInDays
}

func (r *CandleRepository) GetLastCandle() *eventmodels.AggregateBarWithIndicators {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if len(r.candlesWithIndicators) == 0 {
		return nil
	}

	return r.candlesWithIndicators[len(r.candlesWithIndicators)-1]
}

func (r *CandleRepository) SetStartingPosition(currentTime time.Time, env PlaygroundEnvironment, calendar *eventmodels.MarketCalendar) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for i, candle := range r.candlesWithIndicators {
		if candle.Timestamp.Equal(currentTime) || candle.Timestamp.After(currentTime) {
			start := i
			r.startingPosition = &start
			r.position = start
			return nil
		}
	}

	if env == PlaygroundEnvironmentSimulator || env == PlaygroundEnvironmentLive {
		start := len(r.candlesWithIndicators) - 1
		r.startingPosition = &start
		r.position = start
		showAlert := true

		// Check if the market is open
		if calendar != nil {
			now := time.Now()
			result, err := eventservices.IsMarketOpen(calendar, now)
			if err != nil {
				return fmt.Errorf("failed to check if market is open: %v", err)
			}
			showAlert = result
		}

		if showAlert {
			startingCandle := r.candlesWithIndicators[start]
			log.Warnf("no candles found at or after %s, but market is open. Setting start candle to %s", currentTime, startingCandle.Timestamp)
		}

		return nil
	}

	return fmt.Errorf("no candles found at or after %s", currentTime)
}

func (r *CandleRepository) AppendBars(bars []eventmodels.ICandle) (time.Time, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if len(bars) == 0 {
		return time.Time{}, nil
	}

	maxTimestamp := time.Time{}
	for i, bar := range bars {
		if !r.candlesWithIndicators[len(r.candlesWithIndicators)-1].Timestamp.Before(bar.GetTimestamp()) {
			return time.Time{}, fmt.Errorf("new bar[%d] is not after the last bar", i)
		}

		r.baseCandles = append(r.baseCandles, &eventmodels.PolygonAggregateBarV2{
			Timestamp: bar.GetTimestamp(),
			Open:      bar.GetOpen(),
			High:      bar.GetHigh(),
			Low:       bar.GetLow(),
			Close:     bar.GetClose(),
			Volume:    bar.GetVolume(),
		})

		if bar.GetTimestamp().After(maxTimestamp) {
			maxTimestamp = bar.GetTimestamp()
		}
	}

	var err error
	previousIndex := len(r.candlesWithIndicators) - 1

	r.candlesWithIndicators, err = eventservices.AddIndicatorsToCandles(r.baseCandles, r.indicators)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to aggregate bars with indicators: %v", err)
	}

	// send new bars to the queue
	if r.newCandlesQueue != nil {
		log.Tracef("sending new (%s, %s) candles to the queue", r.symbol, r.periodStr)
		for i := previousIndex + 1; i < len(r.candlesWithIndicators); i++ {
			r.newCandlesQueue.Enqueue(&BacktesterCandle{
				Symbol: r.symbol,
				Period: r.period,
				Bar:    r.candlesWithIndicators[i],
			})
		}
		log.Tracef("sent %d new (%s, %s) candles to the queue", len(r.candlesWithIndicators)-previousIndex-1, r.symbol, r.periodStr)
	}

	return maxTimestamp, nil
}

func (r *CandleRepository) FetchCandles(startTime time.Time, endTime *time.Time) ([]*eventmodels.AggregateBarWithIndicators, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var candles []*eventmodels.AggregateBarWithIndicators
	for _, candle := range r.candlesWithIndicators {
		if (candle.Timestamp.Equal(startTime) || candle.Timestamp.After(startTime)) && (endTime == nil || candle.Timestamp.Before(*endTime)) {
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

func (r *CandleRepository) getCurrentCandle() (*eventmodels.AggregateBarWithIndicators, error) {
	if r.position >= len(r.candlesWithIndicators) {
		return nil, fmt.Errorf("no more candles")
	}

	if r.startingPosition == nil {
		return nil, nil
	}

	if r.position >= *r.startingPosition {
		return r.candlesWithIndicators[r.position], nil
	}

	return nil, nil
}

func (r *CandleRepository) GetCurrentCandle() (*eventmodels.AggregateBarWithIndicators, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.getCurrentCandle()
}

func (r *CandleRepository) Update(currentTime time.Time) (*eventmodels.AggregateBarWithIndicators, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.position >= len(r.candlesWithIndicators) {
		return nil, fmt.Errorf("no more candles")
	}

	var newCandle *eventmodels.AggregateBarWithIndicators
	var err error
	for {
		if r.position >= len(r.candlesWithIndicators)-1 {
			break
		}

		nextCandleTimestamp := r.candlesWithIndicators[r.position+1].Timestamp
		if currentTime.Equal(nextCandleTimestamp) || currentTime.After(nextCandleTimestamp) {
			r.position++
			r.isInitialTick = false
			newCandle, err = r.getCurrentCandle()
			if err != nil {
				return nil, fmt.Errorf("failed to get current candle: %v", err)
			}
		} else if r.isInitialTick {
			if !(currentTime.Before(r.candlesWithIndicators[r.position].Timestamp)) {
				newCandle, err = r.getCurrentCandle()
				if err != nil {
					return nil, fmt.Errorf("failed to get current candle during initial tick: %v", err)
				}
			}

			r.isInitialTick = false
			break
		} else {
			break
		}
	}

	return newCandle, nil
}

func NewCandleRepository(symbol eventmodels.Instrument, period time.Duration, candles []*eventmodels.PolygonAggregateBarV2, indicators []string, newCandlesQueue *eventmodels.FIFOQueue[*BacktesterCandle], historyInDays uint32, source eventmodels.CandleRepositorySource) (*CandleRepository, error) {
	var interval eventmodels.TradierInterval
	switch period {
	case time.Minute:
		interval = eventmodels.TradierInterval1Min
	case 5 * time.Minute:
		interval = eventmodels.TradierInterval5Min
	case 15 * time.Minute:
		interval = eventmodels.TradierInterval15Min
	default:
		if period%(15*time.Minute) != 0 {
			return nil, fmt.Errorf("period must be a multiple of 15 minutes: %s", period)
		}

		interval = eventmodels.TradierInterval15Min
	}

	polygonTimespan, err := eventmodels.NewPolygonTimespanRequest(period)
	if err != nil {
		return nil, fmt.Errorf("failed to create polygon timespan: %v", err)
	}

	var candlesWithIndicators []*eventmodels.AggregateBarWithIndicators
	if len(indicators) > 0 {
		candlesWithIndicators, err = eventservices.AddIndicatorsToCandles(candles, indicators)
		if err != nil {
			return nil, fmt.Errorf("failed to add indicators to candles: %v", err)
		}
	} else {
		for _, candle := range candles {
			candlesWithIndicators = append(candlesWithIndicators, &eventmodels.AggregateBarWithIndicators{
				Volume:    candle.Volume,
				Open:      candle.Open,
				High:      candle.High,
				Low:       candle.Low,
				Close:     candle.Close,
				Timestamp: candle.Timestamp,
			})
		}
	}

	log.Debugf("adding newCandlesQueue(%p) to CandleRepository (%s, %s)", newCandlesQueue, symbol, period.String())

	return &CandleRepository{
		symbol:                symbol,
		period:                period,
		periodStr:             period.String(),
		fetchInterval:         interval,
		candlesWithIndicators: candlesWithIndicators,
		baseCandles:           candles,
		indicators:            indicators,
		newCandlesQueue:       newCandlesQueue,
		polygonTimespan:       polygonTimespan,
		isInitialTick:         true,
		historyInDays:         historyInDays,
		source:                source,
		mutex:                 &sync.Mutex{},
	}, nil
}
