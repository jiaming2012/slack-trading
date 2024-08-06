package run

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type RunArgs struct {
	StartsAt              time.Time
	EndsAt                time.Time
	Ticker                eventmodels.StockSymbol
	LookaheadCandlesCount []int
	GoEnv                 string
}
