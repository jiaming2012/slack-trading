package eventconsumers

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
)

func SendHighestEVTradeToMarket(ctx context.Context, resultCh chan map[string]interface{}, errCh chan error, event eventmodels.SignalTriggeredEvent, tradierOrderExecuter *eventmodels.TradierOrderExecuter, riskProfileConstraint *eventmodels.RiskProfileConstraint, maxNoOfPositions int, goEnv string) error {
	tracer := otel.GetTracerProvider().Tracer("SendHighestEVTradeToMarket")
	ctx, span := tracer.Start(ctx, "SendHighestEVTradeToMarket")
	defer span.End()

	highestEVOrderComponents, err := eventservices.DeriveHighestEVOrders(ctx, resultCh, errCh, event, tradierOrderExecuter, riskProfileConstraint)
	if err != nil {
		return fmt.Errorf("DeriveHighestEVOrders: failed to derive highest EV orders: %w", err)
	}

	for _, order := range highestEVOrderComponents {
		tradeRequest := eventmodels.PlaceTradeSpreadRequest{
			Underlying:       event.Symbol,
			Spread:           order.Spread,
			Quantity:         1,
			TradeType:        eventmodels.TradierTradeTypeCredit,
			Price:            order.RequestedPrice,
			TradeDuration:    eventmodels.TradeDurationDay,
			Tag:              order.Tag,
			MaxNoOfPositions: maxNoOfPositions,
		}

		// If the symbol is SPX, the minimum preice increment is 0.05
		if event.Symbol == "SPX" {
			tradeRequest.Price = eventservices.RoundToNearest(tradeRequest.Price, 0.05)
		}

		if err := eventservices.PlaceTradeSpread(ctx, tradierOrderExecuter, tradeRequest); err != nil {
			return fmt.Errorf("tradierOrderExecuter.PlaceTradeSpread:: error placing trade: %v", err)
		}
	}

	return nil
}
