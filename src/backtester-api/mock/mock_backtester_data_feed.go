package mock

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type MockBacktesterDataFeed struct {
	symbol eventmodels.Instrument
	period time.Duration
	bars   []*eventmodels.AggregateBarWithIndicators
}

func (feed *MockBacktesterDataFeed) SetStartingPosition(currentTime time.Time) {
	// do nothing
}

func (feed *MockBacktesterDataFeed) FetchCandles(startTime, endTime time.Time) ([]*eventmodels.AggregateBarWithIndicators, error) {
	return feed.bars, nil
}

func (feed *MockBacktesterDataFeed) GetSymbol() eventmodels.Instrument {
	return feed.symbol
}

func (feed *MockBacktesterDataFeed) GetPeriod() time.Duration {
	return feed.period
}

func (feed *MockBacktesterDataFeed) GetSource() string {
	return "mock"
}

func NewMockBacktesterDataFeed(symbol eventmodels.Instrument, period time.Duration, timestamps []time.Time, closes []float64) *MockBacktesterDataFeed {
	if len(timestamps) != len(closes) {
		panic("timestamps and closes must have the same length")
	}

	bars := make([]*eventmodels.AggregateBarWithIndicators, len(closes))
	for i := 0; i < len(closes); i++ {
		bars[i] = &eventmodels.AggregateBarWithIndicators{
			Timestamp: timestamps[i],
			Close:     closes[i],
		}
	}

	return &MockBacktesterDataFeed{
		symbol: symbol,
		period: period,
		bars:   bars,
	}
}
