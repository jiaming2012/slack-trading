package services

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"

	"github.com/jiaming2012/slack-trading/src/cmd/fetch_orders/run"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func fetchCandles(symbol eventmodels.StockSymbol, from, to time.Time, apiKey string) ([]*eventmodels.CandleDTO, error) {
	resp, err := eventservices.FetchPolygonStockChart(symbol, 1, "minute", from, to, apiKey)
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

func FetchCandlesFromOrderRecords(symbol eventmodels.StockSymbol, orders []*eventmodels.OrderRecord, apiKey string) ([]*eventmodels.CandleDTO, error) {
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

	openCandles, err := fetchCandles(symbol, firstOpenTime, finalOpenTime, apiKey)
	if err != nil {
		return nil, fmt.Errorf("fetchCandles: failed to fetch open candles: %v", err)
	}

	expirationCandles, err := fetchCandles(symbol, firstExpiration, finalExpiration, apiKey)
	if err != nil {
		return nil, fmt.Errorf("fetchCandles: failed to fetch expiration candles: %v", err)
	}

	results = append(results, openCandles...)
	results = append(results, expirationCandles...)

	return results, nil
}

func ProcessBacktestTrades(symbol eventmodels.StockSymbol, orders []*eventmodels.OrderRecord, candles []*eventmodels.CandleDTO, outDir string) (string, error) {
	var spreadResults []*eventmodels.OptionOrderSpreadResult
	optionMultiplier := 100.0

	for i, order := range orders {
		req := eventmodels.OptionSpreadAnalysisRequest{
			ID:            uint(i),
			Underlying:    symbol,
			ExecutionType: "market",
			CreateDate:    eventmodels.GetMinTime(order.Spread.LongOptionTimestamp, order.Spread.ShortOptionTimestamp),
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
			Config:       order.Config,
		}

		result, err := utils.CalculateOptionOrderSpreadResult(req, candles, optionMultiplier)
		if err != nil {
			return "", fmt.Errorf("failed to calculate option order spread result: %w", err)
		}

		spreadResults = append(spreadResults, result)
	}

	csvPath, err := run.ExportToCsv(outDir, spreadResults, fmt.Sprintf("backtester_%s", symbol))
	if err != nil {
		return "", fmt.Errorf("failed to export to CSV: %w", err)
	}

	return csvPath, nil
}

func DeriveHighestEVOrderRecord(ctx context.Context, resultCh chan map[string]interface{}, errCh chan error, event eventmodels.SignalTriggeredEvent, tradierOrderExecuter *eventmodels.TradierOrderExecuter, config *eventmodels.OptionYAML, riskProfileConstraint *eventmodels.RiskProfileConstraint, goEnv string) ([]*eventmodels.OrderRecord, error) {
	tracer := otel.GetTracerProvider().Tracer("SendHighestEVTradeToMarket")
	ctx, span := tracer.Start(ctx, "SendHighestEVTradeToMarket")
	defer span.End()

	highestEVOrderComponents, err := eventservices.DeriveHighestEVOrders(ctx, resultCh, errCh, event, tradierOrderExecuter, riskProfileConstraint)
	if err != nil {
		return nil, fmt.Errorf("DeriveHighestEVOrders: failed to derive highest EV orders: %w", err)
	}

	var results []*eventmodels.OrderRecord
	for _, order := range highestEVOrderComponents {
		results = append(results, &eventmodels.OrderRecord{
			Underlying: event.Symbol,
			Spread:     order.Spread,
			Quantity:   1,
			Tag:        order.Tag,
			Config:     config,
		})
	}

	return results, nil
}
