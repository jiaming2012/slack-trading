package mock

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type MockBacktesterDataFeed struct {
	symbol eventmodels.Instrument
	bars   []*eventmodels.PolygonAggregateBarV2
}

func (feed *MockBacktesterDataFeed) FetchCandles(startTime, endTime time.Time) ([]*eventmodels.PolygonAggregateBarV2, error) {
	return feed.bars, nil
}

func (feed *MockBacktesterDataFeed) GetSymbol() eventmodels.Instrument {
	return feed.symbol
}

func NewMockBacktesterDataFeed(symbol eventmodels.Instrument, timestamps []time.Time, closes []float64) *MockBacktesterDataFeed {
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
		bars:   bars,
	}
}
