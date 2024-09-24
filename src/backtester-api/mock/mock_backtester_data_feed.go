package mock

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type MockBacktesterDataFeed struct {
	currentPrice float64
}

func (feed *MockBacktesterDataFeed) SetCurrentPrice(price float64) {
	feed.currentPrice = price
}

func (feed *MockBacktesterDataFeed) FetchStockPrice(time time.Time, symbol eventmodels.StockSymbol) (float64, error) {
	return feed.currentPrice, nil
}

func NewMockBacktesterDataFeed() *MockBacktesterDataFeed {
	return &MockBacktesterDataFeed{}
}
