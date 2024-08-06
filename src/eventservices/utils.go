package eventservices

import (
	"fmt"
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

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

func FindClosestStockTickItemDTO(req eventmodels.PolygonDataBulkHistOptionOHLCRequest, at time.Time, spreadPerc float64) (*eventmodels.StockTickItemDTO, error) {
	resp, err := FetchPolygonStockChart(req.Root, 1, "minute", at, at.AddDate(0, 0, 1))
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
