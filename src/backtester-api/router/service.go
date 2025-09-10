package router

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func (s Server) fetchOptionCandles(playgroundID uuid.UUID, symbol eventmodels.OptionSymbol, period time.Duration, from time.Time, to *time.Time) ([]*eventmodels.AggregateBarWithIndicators, error) {
	result, err := s.optionsClient.FetchPolygonOptionAggregateBars(playgroundID, symbol, period, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetchOptionCandles: failed to fetch option candles: %w", err)
	}

	var candles []*eventmodels.AggregateBarWithIndicators
	for _, bar := range result.Results {
		timestamp := time.Unix(int64(bar.Time), 0)
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

func (s Server) fetchCandles(playgroundID uuid.UUID, symbol eventmodels.StockSymbol, period time.Duration, from time.Time, to *time.Time) ([]*eventmodels.AggregateBarWithIndicators, error) {
	playground, err := s.dbService.GetPlayground(playgroundID)
	if err != nil {
		return nil, eventmodels.NewWebError(404, "handleCandles: playground not found", nil)
	}

	candles, err := playground.FetchCandles(symbol, period, from, to)
	if err != nil {
		return nil, eventmodels.NewWebError(500, "failed to fetch candles", err)
	}

	return candles, nil
}

func (s Server) nextTick(playgroundID uuid.UUID, duration time.Duration, isPreview bool) (*models.TickDelta, error) {
	playground, err := s.dbService.GetPlayground(playgroundID)
	if err != nil {
		return nil, fmt.Errorf("playground not found")
	}

	tickDelta, err := playground.Tick(duration, isPreview)
	if err != nil {
		return nil, fmt.Errorf("failed to tick: %v", err)
	}

	// Close expired option contracts repos
	executionRequests := make(map[uint]models.ExecutionFillRequest)
	for _, event := range tickDelta.Events {
		if event.Type == models.TickDeltaEventTypeOptionExpired {
			openOrders := playground.GetOpenOrders(event.ExpiredOptionContractEvent.Symbol)
			for _, o := range openOrders {
				components, err := event.ExpiredOptionContractEvent.Symbol.Components()
				if err != nil {
					return nil, fmt.Errorf("failed to get symbol components: %w", err)
				}

				var closePriceAtExpiration float64
				if components.OptionType == eventmodels.OptionTypeCall {
					if event.ExpiredOptionContractEvent.UnderlyingPriceAtExpiry > components.StrikePrice {
						closePriceAtExpiration = event.ExpiredOptionContractEvent.UnderlyingPriceAtExpiry - components.StrikePrice
					} else {
						closePriceAtExpiration = 0
					}
				} else if components.OptionType == eventmodels.OptionTypePut {
					if event.ExpiredOptionContractEvent.UnderlyingPriceAtExpiry < components.StrikePrice {
						closePriceAtExpiration = components.StrikePrice - event.ExpiredOptionContractEvent.UnderlyingPriceAtExpiry
					} else {
						closePriceAtExpiration = 0
					}
				} else {
					return nil, fmt.Errorf("unknown option type: %s", components.OptionType)
				}

				closeOrderRequest, err := o.CreateCloseOrderRequest(playground.GetCurrentTime(), closePriceAtExpiration, "auto-closed-on-expiration")
				if err != nil {
					return nil, fmt.Errorf("failed to create close order request: %w", err)
				}

				closeOrder, closeErr := s.dbService.PlaceOrder(playgroundID, closeOrderRequest)
				if closeErr != nil {
					return nil, fmt.Errorf("failed to place close order: %w", closeErr)
				}

				executionRequests[closeOrder.ID] = models.ExecutionFillRequest{
					Price:    closeOrder.RequestedPrice,
					Time:     playground.GetCurrentTime(),
					Quantity: closeOrder.GetQuantity(),
				}
			}

			playground.DeleteRepository(event.ExpiredOptionContractEvent.Symbol)
		}
	}

	newTrade, invalidOrders, _, err := playground.CommitOrderQueue(executionRequests)
	if err != nil {
		return nil, fmt.Errorf("failed to commit order queue: %w", err)
	}

	tickDelta.NewTrades = append(tickDelta.NewTrades, newTrade...)
	tickDelta.InvalidOrders = append(tickDelta.InvalidOrders, invalidOrders...)

	if playground.GetMeta().Environment == models.PlaygroundEnvironmentLive {
		if err := s.dbService.SaveEquityPlotRecord(playgroundID, tickDelta.EquityPlot.Timestamp, tickDelta.EquityPlot.Value); err != nil {
			return nil, fmt.Errorf("failed to save equity plot record: %v", err)
		}
	}

	return tickDelta, nil
}
