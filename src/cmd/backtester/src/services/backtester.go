package services

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/jiaming2012/slack-trading/src/cmd/fetch_orders/run"
	"github.com/jiaming2012/slack-trading/src/eventconsumers"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func fetchCandles(symbol eventmodels.StockSymbol, from, to time.Time) ([]*eventmodels.CandleDTO, error) {
	resp, err := eventservices.FetchPolygonStockChart(symbol, 1, "minute", from, to)
	if err != nil {
		return nil, fmt.Errorf("fetchCandles: failed to fetch stock chart: %v", err)
	}

	var candles []*eventmodels.CandleDTO
	for _, c := range resp.Results {
		dto, err := c.ToCandleDTO()
		if err != nil {
			return nil, fmt.Errorf("fetchCandles: failed to convert to candle dto: %v", err)
		}

		candles = append(candles, dto)
	}

	return candles, nil
}

func FetchCandlesFromBacktesterOrders(symbol eventmodels.StockSymbol, orders []*eventmodels.BacktesterOrder) ([]*eventmodels.CandleDTO, error) {
	var firstOpenTime, finalOpenTime time.Time
	var firstExpiration, finalExpiration time.Time
	var results []*eventmodels.CandleDTO

	for _, o := range orders {
		exp, err := o.Spread.GetExpiration()
		if err != nil {
			return nil, fmt.Errorf("fetchCandles: failed to get expiration: %v", err)
		}

		if firstOpenTime.IsZero() || o.Spread.Timestamp.Before(firstOpenTime) {
			firstOpenTime = o.Spread.Timestamp
		}

		if finalOpenTime.IsZero() || o.Spread.Timestamp.After(finalOpenTime) {
			finalOpenTime = o.Spread.Timestamp
		}

		if firstExpiration.IsZero() || exp.Before(firstExpiration) {
			firstExpiration = exp
		}

		if finalExpiration.IsZero() || exp.After(finalExpiration) {
			finalExpiration = exp
		}
	}

	openCandles, err := fetchCandles(symbol, firstOpenTime, finalOpenTime)
	if err != nil {
		return nil, fmt.Errorf("fetchCandles: failed to fetch open candles: %v", err)
	}

	expirationCandles, err := fetchCandles(symbol, firstExpiration, finalExpiration)
	if err != nil {
		return nil, fmt.Errorf("fetchCandles: failed to fetch expiration candles: %v", err)
	}

	results = append(results, openCandles...)
	results = append(results, expirationCandles...)

	return results, nil
}

func ProcessBacktestTrades(symbol eventmodels.StockSymbol, orders []*eventmodels.BacktesterOrder, candles []*eventmodels.CandleDTO, outDir string) error {
	var spreadResults []*eventmodels.OptionOrderSpreadResult
	optionMultiplier := 100.0

	for i, order := range orders {
		req := eventmodels.OptionSpreadAnalysisRequest{
			ID:            uint(i),
			Underlying:    symbol,
			ExecutionType: "market",
			CreateDate:    utils.GetMinTime(order.Spread.LongOptionTimestamp, order.Spread.ShortOptionTimestamp),
			Leg1: eventmodels.OptionSpreadLeg{
				ID:           0,
				Timestamp:    order.Spread.ShortOptionTimestamp,
				Symbol:       order.Spread.ShortOptionSymbol,
				Side:         "sell_to_open",
				Quantity:     1,
				AvgFillPrice: order.Spread.ShortOptionAvgFillPrice,
			},
			Leg2: eventmodels.OptionSpreadLeg{
				ID:           0,
				Timestamp:    order.Spread.LongOptionTimestamp,
				Symbol:       order.Spread.LongOptionSymbol,
				Side:         "buy_to_open",
				Quantity:     1,
				AvgFillPrice: order.Spread.LongOptionAvgFillPrice,
			},
			Tag:          order.Tag,
			AvgFillPrice: *order.Spread.CreditReceived * -1,
		}

		result, err := utils.CalculateOptionOrderSpreadResult(req, candles, optionMultiplier)
		if err != nil {
			return fmt.Errorf("failed to calculate option order spread result: %v", err)
		}

		spreadResults = append(spreadResults, result)
	}

	csvPath, err := run.ExportToCsv(outDir, spreadResults, fmt.Sprintf("backtester_%s", symbol))
	if err != nil {
		log.Errorf("Failed to export to CSV: %v", err)
	} else {
		log.Infof("CSV file written to: %v", csvPath)
	}

	return nil
}

func DeriveHighestEVBacktesterOrder(ctx context.Context, resultCh chan map[string]interface{}, errCh chan error, event eventconsumers.SignalTriggeredEvent, tradierOrderExecuter *eventmodels.TradierOrderExecuter, goEnv string) (*eventmodels.BacktesterOrder, error) {
	tracer := otel.GetTracerProvider().Tracer("SendHighestEVTradeToMarket")
	ctx, span := tracer.Start(ctx, "SendHighestEVTradeToMarket")
	defer span.End()

	logger := log.WithContext(ctx)

	select {
	case result := <-resultCh:
		if result != nil {
			options, ok := result["options"].(map[string][]*eventmodels.OptionSpreadContractDTO)
			if !ok {
				return nil, fmt.Errorf("options not found in result")
			}

			if calls, ok := options["calls"]; ok {
				highestEVLongCallSpreads, highestEVShortCallSpreads, err := eventconsumers.FindHighestEVPerExpiration(calls)
				if err != nil {
					return nil, fmt.Errorf("FindHighestEVPerExpiration: failed to find highest EV per expiration: %w", err)
				}

				for _, spread := range highestEVLongCallSpreads {
					if spread != nil {
						logger.WithField("event", "signal").Infof("Ignoring long call: %v", spread)
					} else {
						logger.WithField("event", "signal").Infof("No Positive EV Long Call found")
					}
				}

				for _, spread := range highestEVShortCallSpreads {
					if spread != nil {
						requestedPrc := 0.0
						if spread.CreditReceived != nil {
							requestedPrc = *spread.CreditReceived
						}

						tag := utils.EncodeTag(event.Signal, spread.Stats.ExpectedProfitShort, requestedPrc)

						span.AddEvent("PlaceTradeSpread:Call", trace.WithAttributes(attribute.String("tag", tag)))
						return &eventmodels.BacktesterOrder{
							Underlying: event.Symbol,
							Spread:     spread,
							Quantity:   1,
							Tag:        tag,
						}, nil
					} else {
						logger.WithField("event", "signal").Infof("No Positive EV Short Call found")
					}
				}
			} else {
				return nil, fmt.Errorf("calls not found in result")
			}

			if puts, ok := options["puts"]; ok {
				highestEVLongPutSpreads, highestEVShortPutSpreads, err := eventconsumers.FindHighestEVPerExpiration(puts)
				if err != nil {
					return nil, fmt.Errorf("FindHighestEVPerExpiration: failed to find highest EV per expiration: %w", err)
				}

				for _, spread := range highestEVLongPutSpreads {
					if spread != nil {
						logger.WithField("event", "signal").Infof("Ignoring long put: %v", spread)
					} else {
						logger.WithField("event", "signal").Infof("No Positive EV Long Put found")
					}
				}

				for _, spread := range highestEVShortPutSpreads {
					if spread != nil {
						requestedPrc := 0.0
						if spread.CreditReceived != nil {
							requestedPrc = *spread.CreditReceived
						}

						tag := utils.EncodeTag(event.Signal, spread.Stats.ExpectedProfitShort, requestedPrc)

						span.AddEvent("PlaceTradeSpread:Put", trace.WithAttributes(attribute.String("tag", tag)))

						return &eventmodels.BacktesterOrder{
							Underlying: event.Symbol,
							Spread:     spread,
							Quantity:   1,
							Tag:        tag,
						}, nil
					} else {
						logger.WithField("event", "signal").Infof("No Positive EV Short Put found")
					}
				}
			} else {
				return nil, fmt.Errorf("puts not found in result")
			}
		}

	case err := <-errCh:
		return nil, fmt.Errorf("error: %v", err)
	}

	return nil, nil
}
