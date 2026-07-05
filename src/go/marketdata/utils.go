package marketdata

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

func FetchOptionCandles(client *PolygonOptionsClient, playgroundID uuid.UUID, symbol models.OptionSymbol, period time.Duration, from time.Time, to *time.Time) ([]*models.AggregateBarWithIndicators, error) {
	result, err := client.FetchPolygonOptionAggregateBars(playgroundID, symbol, period, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetchOptionCandles: failed to fetch option candles: %w", err)
	}

	var candles []*models.AggregateBarWithIndicators
	for _, bar := range result.Results {
		timestamp := time.UnixMilli(int64(bar.Time))
		candles = append(candles, &models.AggregateBarWithIndicators{
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

func findClosestPriceBeforeOrAt(candles []*models.Candle, at time.Time) (float64, error) {
	var closestCandle *models.Candle
	for _, candle := range candles {
		if candle.Timestamp.After(at) {
			break
		}

		closestCandle = candle
	}

	if closestCandle == nil {
		return 0, fmt.Errorf("no candle found before or at %v", at)
	}

	return closestCandle.Open, nil
}

// func transformSPYtoSPX(result *models.StockTickItemDTO) {
// 	if result.Symbol == "SPY" {
// 		result.Symbol = "SPX"
// 		result.Bid *=  10
// 		result.Ask *= 10
// 	}
// }

func FindClosestStockTickItemDTO(req models.PolygonDataBulkHistOptionOHLCRequest, at time.Time, spreadPerc float64) (*models.StockTickItemDTO, error) {
	// if req.Root == "SPX" {
	// 	req.Root = "SPY"
	// }

	result, err := findClosestStockTickItemDTO(req, at, spreadPerc)

	// if req.Root == "SPY" {
	// 	transformSPYtoSPX(result)
	// }

	return result, err
}

func findClosestStockTickItemDTO(req models.PolygonDataBulkHistOptionOHLCRequest, at time.Time, spreadPerc float64) (*models.StockTickItemDTO, error) {
	var targetPrice *float64
	maxAttempts := 10

	for attempt := 0; attempt < maxAttempts; attempt++ {
		resp, err := FetchPolygonStockChart(req.Root, 1, "minute", at.AddDate(0, 0, -1), at.AddDate(0, 0, 1), req.ApiKey)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch underlying price near close: %w", err)
		}

		var candlesNearPriceDTO []*models.CandleDTO
		for _, c := range resp.Results {
			dto, err := c.ToCandleDTO()
			if err != nil {
				return nil, fmt.Errorf("failed to convert to candle dto: %w", err)
			}

			candlesNearPriceDTO = append(candlesNearPriceDTO, dto)
		}

		var candles []*models.Candle
		for _, dto := range candlesNearPriceDTO {
			c, err := dto.ToCandle(time.UTC)
			if err != nil {
				return nil, fmt.Errorf("failed to convert dto to candle: %w", err)
			}

			candles = append(candles, &c)
		}

		closestPrice, err := findClosestPriceBeforeOrAt(candles, at)
		if err != nil {
			log.Debugf("attempt %d: failed to find closest price before or at %v: %v", attempt+1, at, err)
			for {
				at = at.AddDate(0, 0, -1)
				if at.Weekday() != time.Saturday && at.Weekday() != time.Sunday {
					break
				}
			}
			
			time.Sleep(50 * time.Millisecond)
			continue
		}

		targetPrice = &closestPrice
		break
	}

	if targetPrice == nil {
		return nil, fmt.Errorf("failed to find closest price before or at %v after %d attempts", at, maxAttempts)
	}

	return &models.StockTickItemDTO{
		Timestamp: at,
		Symbol:    string(req.Root),
		Bid:       *targetPrice,
		Ask:       *targetPrice * (1 + spreadPerc),
	}, nil
}
