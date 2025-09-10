package eventservices

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func FetchOptionCandles(client *PolygonOptionsClient, playgroundID uuid.UUID, symbol eventmodels.OptionSymbol, period time.Duration, from time.Time, to *time.Time) ([]*eventmodels.AggregateBarWithIndicators, error) {
	result, err := client.FetchPolygonOptionAggregateBars(playgroundID, symbol, period, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetchOptionCandles: failed to fetch option candles: %w", err)
	}

	var candles []*eventmodels.AggregateBarWithIndicators
	for _, bar := range result.Results {
		timestamp := time.UnixMilli(int64(bar.Time))
		candles = append(candles, &eventmodels.AggregateBarWithIndicators{
			Timestamp: timestamp,
			Open:      bar.Open,
			Close:     bar.Close,
			High:      bar.High,
			Low:       bar.Low,
		})
	}

	// todo: add a cache for the candles

	return candles, nil
}

func findClosestPriceBeforeOrAt(candles []*eventmodels.Candle, at time.Time) (float64, error) {
	var closestCandle *eventmodels.Candle
	for _, candle := range candles {
		if candle.Timestamp.After(at) {
			break
		}

		closestCandle = candle
	}

	return closestCandle.Open, nil
}

// func transformSPYtoSPX(result *eventmodels.StockTickItemDTO) {
// 	if result.Symbol == "SPY" {
// 		result.Symbol = "SPX"
// 		result.Bid *=  10
// 		result.Ask *= 10
// 	}
// }

func FindClosestStockTickItemDTO(req eventmodels.PolygonDataBulkHistOptionOHLCRequest, at time.Time, spreadPerc float64) (*eventmodels.StockTickItemDTO, error) {
	// if req.Root == "SPX" {
	// 	req.Root = "SPY"
	// }

	result, err := findClosestStockTickItemDTO(req, at, spreadPerc)

	// if req.Root == "SPY" {
	// 	transformSPYtoSPX(result)
	// }

	return result, err
}

func findClosestStockTickItemDTO(req eventmodels.PolygonDataBulkHistOptionOHLCRequest, at time.Time, spreadPerc float64) (*eventmodels.StockTickItemDTO, error) {
	resp, err := FetchPolygonStockChart(req.Root, 1, "minute", at.AddDate(0, 0, -1), at.AddDate(0, 0, 1), req.ApiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch underlying price near close: %w", err)
	}

	var candlesNearPriceDTO []*eventmodels.CandleDTO
	for _, c := range resp.Results {
		dto, err := c.ToCandleDTO()
		if err != nil {
			return nil, fmt.Errorf("failed to convert to candle dto: %w", err)
		}

		candlesNearPriceDTO = append(candlesNearPriceDTO, dto)
	}

	var candles []*eventmodels.Candle
	for _, dto := range candlesNearPriceDTO {
		c, err := dto.ToCandle(time.UTC)
		if err != nil {
			return nil, fmt.Errorf("failed to convert dto to candle: %w", err)
		}

		candles = append(candles, &c)
	}

	closestPrice, err := findClosestPriceBeforeOrAt(candles, at)
	if err != nil {
		return nil, fmt.Errorf("failed to find closest candle: %w", err)
	}

	return &eventmodels.StockTickItemDTO{
		Timestamp: at,
		Symbol:    string(req.Root),
		Bid:       closestPrice,
		Ask:       closestPrice * (1 + spreadPerc),
	}, nil
}
