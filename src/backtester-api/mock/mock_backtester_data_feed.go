package mock

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type MockBacktesterDataFeed struct {
	symbol eventmodels.Instrument
	period time.Duration
	bars   []*eventmodels.PolygonAggregateBarV2
}

func (feed *MockBacktesterDataFeed) FetchCandles(period time.Duration, startTime, endTime time.Time) ([]*eventmodels.PolygonAggregateBarV2, error) {
	return feed.bars, nil
}

func (feed *MockBacktesterDataFeed) GetSymbol() eventmodels.Instrument {
	return feed.symbol
}

func (feed *MockBacktesterDataFeed) GetPeriod() time.Duration {
	return feed.period
}

func NewMockBacktesterDataFeed(symbol eventmodels.Instrument, period time.Duration, timestamps []time.Time, closes []float64) *MockBacktesterDataFeed {
	if len(timestamps) != len(closes) {
		panic("timestamps and closes must have the same length")
	}

	bars := make([]*eventmodels.PolygonAggregateBarV2, len(closes))
	for i := 0; i < len(closes); i++ {
		bars[i] = &eventmodels.PolygonAggregateBarV2{
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
