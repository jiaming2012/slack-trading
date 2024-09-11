package eventmodels

import (
	"context"
	"time"
)

// OptionsDataFetcher is an interface for fetching options data
// both real-time and historical
type OptionsDataFetcher interface {
	FetchEVSpreads(ctx context.Context, projectDir string, signalName SignalName, bFindSpreads bool, startsAt, endsAt time.Time, ticker StockSymbol, goEnv string, options []OptionContractV3, stockInfo *StockTickItemDTO, now time.Time) (map[string]ExpectedProfitItemSpread, map[string]ExpectedProfitItemSpread, error)
	FetchOptionChainDataInput(symbol StockSymbol, isHistorical bool, timestamp time.Time, expirationGTE, expirationLTE time.Time, maxNoOfStrikes int, minDistanceBetweenStrikes float64, expirationInDays []int) (*FetchOptionChainDataInput, error)
}
