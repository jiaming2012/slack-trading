package workers

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"

	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/marketdata"
)

func SendHighestEVTradeToMarket(ctx context.Context, resultCh chan map[string]interface{}, errCh chan error, event models.SignalTriggeredEvent, tradierOrderExecuter *models.TradierOrderExecuter, riskProfileConstraint *models.RiskProfileConstraint, maxNoOfPositions int, goEnv string) error {
	tracer := otel.GetTracerProvider().Tracer("SendHighestEVTradeToMarket")
	ctx, span := tracer.Start(ctx, "SendHighestEVTradeToMarket")
	defer span.End()

	highestEVOrderComponents, err := marketdata.DeriveHighestEVOrders(ctx, resultCh, errCh, event, tradierOrderExecuter, riskProfileConstraint)
	if err != nil {
		return fmt.Errorf("SendHighestEVTradeToMarket: failed to derive highest EV orders: %w", err)
	}

	for _, order := range highestEVOrderComponents {
		tradeRequest := models.PlaceTradeSpreadRequest{
			Underlying:       event.Symbol,
			Spread:           order.Spread,
			Quantity:         1,
			TradeType:        models.TradierOrderTypeCredit,
			Price:            order.RequestedPrice,
			TradeDuration:    models.TradeDurationDay,
			Tag:              order.Tag,
			MaxNoOfPositions: maxNoOfPositions,
		}

		if err := marketdata.CheckMaxNoOfPositions(tradierOrderExecuter, tradeRequest.Underlying, tradeRequest.Quantity, tradeRequest.MaxNoOfPositions); err != nil {
			return fmt.Errorf("SendHighestEVTradeToMarket: failed to check max no of positions: %w", err)
		}

		if err := marketdata.PlaceTradeSpread(ctx, tradierOrderExecuter, tradeRequest); err != nil {
			return fmt.Errorf("SendHighestEVTradeToMarket: error placing trade: %v", err)
		}
	}

	return nil
}
