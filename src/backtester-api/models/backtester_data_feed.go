package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type BacktesterDataFeed interface {
	FetchStockPrice(time time.Time, symbol eventmodels.StockSymbol) (float64, error)
}
