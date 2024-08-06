package eventmodels

import "time"

type SupertrendRunArgs struct {
	StartsAt              time.Time
	EndsAt                time.Time
	Ticker                StockSymbol
	LookaheadCandlesCount []int
	GoEnv                 string
}
