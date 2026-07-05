package models

import (
	"fmt"
	"sort"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/eventservices"
)

type CandleRepository struct {
	symbol                models.Instrument
	period                time.Duration
	periodStr             string
	fetchInterval         models.TradierInterval
	polygonTimespan       models.PolygonTimespan
	candlesWithIndicators []*models.AggregateBarWithIndicators
	baseCandles           []*models.PolygonAggregateBarV2
	indicators            []string
	position              int
	startingPosition      *int
	newCandlesQueue       *models.FIFOQueue[*BacktesterCandle]
	isInitialTick         bool
	historyInDays         uint32
	nextUpdateAt          *time.Time
	source                models.CandleRepositorySource
	mutex                 *sync.Mutex
	optionComponents      *models.OptionSymbolComponents
}

func (r *CandleRepository) Count() int {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return len(r.candlesWithIndicators)
}

func (r *CandleRepository) Sort() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	sort.Slice(r.candlesWithIndicators, func(i, j int) bool {
		return r.candlesWithIndicators[i].Timestamp.Before(r.candlesWithIndicators[j].Timestamp)
	})
}

func (r *CandleRepository) AddCandles(candles []*models.AggregateBarWithIndicators) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	switch r.symbol.(type) {
	case models.OptionSymbol:
	case *models.OptionContractV3:
	default:
		return fmt.Errorf("AddCandles: unsupported symbol type: %T", r.symbol)
	}

	r.candlesWithIndicators = append(r.candlesWithIndicators, candles...)

	sort.Slice(r.candlesWithIndicators, func(i, j int) bool {
		return r.candlesWithIndicators[i].Timestamp.Before(r.candlesWithIndicators[j].Timestamp)
	})

	return nil
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

func (r *CandleRepository) GetSymbol() models.Instrument {
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

func (r *CandleRepository) GetPolygonTimespan() models.PolygonTimespan {
	return r.polygonTimespan
}

func (r *CandleRepository) GetFetchInterval() models.TradierInterval {
	return r.fetchInterval
}

func (r *CandleRepository) GetHistoryInDays() uint32 {
	return r.historyInDays
}

func (r *CandleRepository) GetLastCandle() *models.AggregateBarWithIndicators {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if len(r.candlesWithIndicators) == 0 {
		return nil
	}

	return r.candlesWithIndicators[len(r.candlesWithIndicators)-1]
}

func (r *CandleRepository) SetStartingPosition(currentTime time.Time, env PlaygroundEnvironment, calendar *models.MarketCalendar) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if len(r.candlesWithIndicators) == 0 {
		return fmt.Errorf("no candles available to set starting position")
	}

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
			log.Warnf("[%s] no candles found at or after %s, but market is open. Setting start candle to %s",
				r.symbol.GetTicker(),
				currentTime.Format("2006-01-02 15:04:05 MST"),
				startingCandle.Timestamp.Format("2006-01-02 15:04:05 MST"),
			)
		}

		return nil
	}

	return fmt.Errorf("no candles found at or after %s", currentTime)
}

func (r *CandleRepository) FetchCandlesAtOrAfter(tstamp time.Time) (*models.AggregateBarWithIndicators, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, candle := range r.candlesWithIndicators {
		if candle.Timestamp.Equal(tstamp) || candle.Timestamp.After(tstamp) {
			return candle, nil
		}
	}

	log.Warnf("No candles found for %s at or after %s", r.symbol, tstamp.Format("2006-01-02 15:04:05 MST"))

	return nil, nil
}

func (r *CandleRepository) AppendBars(bars []models.ICandle) (time.Time, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if len(bars) == 0 {
		return time.Time{}, nil
	}

	maxTimestamp := time.Time{}
	for i, bar := range bars {
		lastBar := r.candlesWithIndicators[len(r.candlesWithIndicators)-1]
		if !lastBar.Timestamp.Before(bar.GetTimestamp()) {
			return time.Time{}, fmt.Errorf("new bar[%d] timestamp %v is not after the last bar timestamp %v (symbol=%s)", i, bar.GetTimestamp(), lastBar.Timestamp, r.symbol)
		}

		r.baseCandles = append(r.baseCandles, &models.PolygonAggregateBarV2{
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

func (r *CandleRepository) FetchCandles(startTime time.Time, endTime *time.Time) ([]*models.AggregateBarWithIndicators, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var candles []*models.AggregateBarWithIndicators
	for _, candle := range r.candlesWithIndicators {
		if (candle.Timestamp.Equal(startTime) || candle.Timestamp.After(startTime)) && (endTime == nil || candle.Timestamp.Before(*endTime)) {
			candles = append(candles, candle)
		}
	}

	if len(candles) == 0 {
		endTimeStr := "<nil>"
		if endTime != nil {
			endTimeStr = endTime.Format("2006-01-02 15:04:05 MST")
		}
		log.Warnf("No candles found for %s between %s and %s", r.symbol, startTime.Format("2006-01-02 15:04:05 MST"), endTimeStr)
	}

	return candles, nil
}

func (r *CandleRepository) GetCandleAt(at time.Time, maxAge time.Duration) (*models.AggregateBarWithIndicators, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, candle := range r.candlesWithIndicators {
		if candle.Timestamp.Equal(at) {
			return candle, nil
		}

		if candle.Timestamp.After(at) {
			if maxAge > 0 && candle.Timestamp.Before(at.Add(-maxAge)) {
				return nil, fmt.Errorf("No candles found for %s at %s within max age %s", r.symbol, at, maxAge)
			}

			return candle, nil
		}
	}

	return nil, fmt.Errorf("No candles found for %s at or after %s", r.symbol, at)
}

func (r *CandleRepository) getCurrentCandle() (*models.AggregateBarWithIndicators, error) {
	if r.position >= len(r.candlesWithIndicators) {
		if r.position == 0 {
			return nil, fmt.Errorf("found empty candlesWithIndicators for symbol %s", r.symbol.GetTicker())
		}
		
		return nil, nil
	}

	if r.startingPosition == nil {
		return nil, nil
	}

	if r.position >= *r.startingPosition {
		return r.candlesWithIndicators[r.position], nil
	}

	return nil, nil
}

func (r *CandleRepository) GetCurrentCandle() (*models.AggregateBarWithIndicators, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.getCurrentCandle()
}

func (r *CandleRepository) Update(currentTime time.Time) (*models.AggregateBarWithIndicators, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.optionComponents != nil {
		if currentTime.After(r.optionComponents.Expiration) {
			return nil, models.ErrOptionContractIsExpired
		}
	}

	if r.position >= len(r.candlesWithIndicators) {
		return nil, fmt.Errorf("no more candles: %w", ErrCurrentPriceNotSet)
	}

	var newCandle *models.AggregateBarWithIndicators
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

func NewCandleRepository(symbol models.Instrument, period time.Duration, candles []*models.PolygonAggregateBarV2, indicators []string, newCandlesQueue *models.FIFOQueue[*BacktesterCandle], historyInDays uint32, source models.CandleRepositorySource) (*CandleRepository, error) {
	var interval models.TradierInterval
	switch period {
	case time.Minute:
		interval = models.TradierInterval1Min
	case 5 * time.Minute:
		interval = models.TradierInterval5Min
	case 15 * time.Minute:
		interval = models.TradierInterval15Min
	default:
		if period%(15*time.Minute) != 0 {
			return nil, fmt.Errorf("period must be a multiple of 15 minutes: %s", period)
		}

		interval = models.TradierInterval15Min
	}

	polygonTimespan, err := models.NewPolygonTimespanRequest(period)
	if err != nil {
		return nil, fmt.Errorf("failed to create polygon timespan: %v", err)
	}

	var candlesWithIndicators []*models.AggregateBarWithIndicators
	if len(indicators) > 0 {
		candlesWithIndicators, err = eventservices.AddIndicatorsToCandles(candles, indicators)
		if err != nil {
			return nil, fmt.Errorf("failed to add indicators to candles: %v", err)
		}
	} else {
		for _, candle := range candles {
			candlesWithIndicators = append(candlesWithIndicators, &models.AggregateBarWithIndicators{
				Volume:    candle.Volume,
				Open:      candle.Open,
				High:      candle.High,
				Low:       candle.Low,
				Close:     candle.Close,
				Timestamp: candle.Timestamp,
			})
		}
	}

	var optionComponents *models.OptionSymbolComponents
	switch optSymbol := symbol.(type) {
	case models.OptionSymbol:
		components, err := optSymbol.Components()
		if err != nil {
			return nil, fmt.Errorf("failed to get option symbol components: %v", err)
		}

		optionComponents = components
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
		optionComponents:      optionComponents,
	}, nil
}
