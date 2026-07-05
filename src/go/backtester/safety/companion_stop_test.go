package safety

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
)

func TestPlaceCompanionStop_LongEntryStopBelowFill(t *testing.T) {
	broker := models.NewMockBroker(1, nil)

	req, err := PlaceCompanionStop(context.Background(), broker, models.ModeMargin, EntryFill{
		Symbol:    "AAPL",
		EntrySide: models.TradierOrderSideBuy,
		Quantity:  10,
		FillPrice: 100.0,
	}, CompanionStopConfig{StopDistance: 5.0})

	require.NoError(t, err)
	require.NotNil(t, req)
	require.Equal(t, models.TradierOrderTypeStop, req.OrderType)
	require.Equal(t, models.TradierOrderSideSell, req.Sides[0], "long protected by a sell stop")
	require.Equal(t, 10, req.Quantities[0])
	require.NotNil(t, req.StopPrice)
	require.InDelta(t, 95.0, *req.StopPrice, 1e-9, "stop 5 below the 100 fill")

	stops := broker.StopOrders()
	require.Len(t, stops, 1)
	require.Equal(t, models.TradierOrderSideSell, stops[0].Sides[0])
	require.InDelta(t, 95.0, *stops[0].StopPrice, 1e-9)
}

func TestPlaceCompanionStop_ShortEntryStopAboveFill(t *testing.T) {
	broker := models.NewMockBroker(1, nil)

	req, err := PlaceCompanionStop(context.Background(), broker, models.ModePaper, EntryFill{
		Symbol:    "AAPL",
		EntrySide: models.TradierOrderSideSellShort,
		Quantity:  7,
		FillPrice: 200.0,
	}, CompanionStopConfig{StopDistance: 8.0})

	require.NoError(t, err)
	require.NotNil(t, req)
	require.Equal(t, models.TradierOrderSideBuyToCover, req.Sides[0], "short protected by a buy_to_cover stop")
	require.Equal(t, 7, req.Quantities[0])
	require.InDelta(t, 208.0, *req.StopPrice, 1e-9, "stop 8 above the 200 fill")

	stops := broker.StopOrders()
	require.Len(t, stops, 1)
	require.InDelta(t, 208.0, *stops[0].StopPrice, 1e-9)
}

func TestPlaceCompanionStop_SimulationPlacesNoStop(t *testing.T) {
	broker := models.NewMockBroker(1, nil)

	req, err := PlaceCompanionStop(context.Background(), broker, models.ModeSimulation, EntryFill{
		Symbol:    "AAPL",
		EntrySide: models.TradierOrderSideBuy,
		Quantity:  10,
		FillPrice: 100.0,
	}, CompanionStopConfig{StopDistance: 5.0})

	require.NoError(t, err)
	require.Nil(t, req, "Simulation must not place a companion stop")
	require.Empty(t, broker.StopOrders())
	require.Empty(t, broker.Requests(), "no order should reach the broker in Simulation")
}

func TestPlaceCompanionStop_NonPositiveDistanceErrors(t *testing.T) {
	for _, dist := range []float64{0, -5} {
		broker := models.NewMockBroker(1, nil)
		req, err := PlaceCompanionStop(context.Background(), broker, models.ModeMargin, EntryFill{
			Symbol:    "AAPL",
			EntrySide: models.TradierOrderSideBuy,
			Quantity:  10,
			FillPrice: 100.0,
		}, CompanionStopConfig{StopDistance: dist})

		require.Error(t, err, "distance %v must error", dist)
		require.Nil(t, req)
		require.Empty(t, broker.StopOrders(), "no stop placed at or through the fill price")
	}
}

func TestPlaceCompanionStop_DistanceThroughZeroErrors(t *testing.T) {
	broker := models.NewMockBroker(1, nil)
	// Distance exceeds the fill price for a long: stop would be <= 0.
	req, err := PlaceCompanionStop(context.Background(), broker, models.ModeMargin, EntryFill{
		Symbol:    "AAPL",
		EntrySide: models.TradierOrderSideBuy,
		Quantity:  10,
		FillPrice: 4.0,
	}, CompanionStopConfig{StopDistance: 5.0})

	require.Error(t, err)
	require.Nil(t, req)
	require.Empty(t, broker.StopOrders())
}

func TestPlaceCompanionStop_UnsupportedEntrySideErrors(t *testing.T) {
	broker := models.NewMockBroker(1, nil)
	req, err := PlaceCompanionStop(context.Background(), broker, models.ModeMargin, EntryFill{
		Symbol:    "AAPL",
		EntrySide: models.TradierOrderSideSell, // a close side, not an entry
		Quantity:  10,
		FillPrice: 100.0,
	}, CompanionStopConfig{StopDistance: 5.0})

	require.Error(t, err)
	require.Nil(t, req)
	require.Empty(t, broker.StopOrders())
}
