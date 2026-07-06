package safety

// Unit tests for companion-stop eligibility, idempotency, and the loud failure
// path (wire-companion-stops). Everything binds the Broker seam to MockBroker;
// no real broker is contacted.

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

func paperMeta() models.Meta {
	return models.Meta{Mode: models.ModePaper, LegacyEnv: "live"}
}

func entryOrder(mutate func(o *models.OrderRecord)) *models.OrderRecord {
	o := &models.OrderRecord{
		Class:            models.OrderRecordClassEquity,
		Side:             models.TradierOrderSideBuy,
		Symbol:           "AAPL",
		AbsoluteQuantity: 10,
	}
	if mutate != nil {
		mutate(o)
	}
	return o
}

func TestCompanionStopEligible_EntryFillsQualify(t *testing.T) {
	require.True(t, CompanionStopEligible(paperMeta(), entryOrder(nil)), "Paper equity buy entry")
	require.True(t, CompanionStopEligible(models.Meta{Mode: models.ModeMargin, LegacyEnv: "live"}, entryOrder(nil)), "Margin equity buy entry")
	require.True(t, CompanionStopEligible(paperMeta(), entryOrder(func(o *models.OrderRecord) {
		o.Side = models.TradierOrderSideSellShort
	})), "sell_short opens a position and qualifies")
}

func TestCompanionStopEligible_EveryExclusion(t *testing.T) {
	closeID := uint(7)

	cases := []struct {
		name  string
		meta  models.Meta
		order *models.OrderRecord
	}{
		{"Simulation mode", models.Meta{Mode: models.ModeSimulation, LegacyEnv: "simulator"}, entryOrder(nil)},
		{"reconciliation container", models.Meta{LegacyEnv: "reconcile"}, entryOrder(nil)},
		{"options class", paperMeta(), entryOrder(func(o *models.OrderRecord) { o.Class = models.OrderRecordClassOption })},
		{"close side (sell)", paperMeta(), entryOrder(func(o *models.OrderRecord) { o.Side = models.TradierOrderSideSell })},
		{"close side (buy_to_cover)", paperMeta(), entryOrder(func(o *models.OrderRecord) { o.Side = models.TradierOrderSideBuyToCover })},
		{"close linkage", paperMeta(), entryOrder(func(o *models.OrderRecord) { o.CloseOrderId = &closeID })},
		{"marked close", paperMeta(), entryOrder(func(o *models.OrderRecord) { o.IsClose = true })},
		{"adjustment", paperMeta(), entryOrder(func(o *models.OrderRecord) { o.IsAdjustment = true })},
		{"system auto-close", paperMeta(), entryOrder(func(o *models.OrderRecord) { o.IsSystemOrder = true })},
		{"companion-stop tag (bare)", paperMeta(), entryOrder(func(o *models.OrderRecord) { o.Tag = CompanionStopTag })},
		{"companion-stop tag (per-entry)", paperMeta(), entryOrder(func(o *models.OrderRecord) { o.Tag = CompanionStopTagForEntry(42) })},
		{"nil order", paperMeta(), nil},
	}

	for _, tc := range cases {
		require.Falsef(t, CompanionStopEligible(tc.meta, tc.order), "%s must be excluded", tc.name)
	}
}

func TestIsCompanionStopOrderTag(t *testing.T) {
	require.True(t, IsCompanionStopOrderTag(CompanionStopTag))
	require.True(t, IsCompanionStopOrderTag(CompanionStopTagForEntry(1)))
	require.False(t, IsCompanionStopOrderTag(""))
	require.False(t, IsCompanionStopOrderTag("strategy-entry"))
	require.False(t, IsCompanionStopOrderTag("companion-stopgap"), "prefix matching must not swallow unrelated tags")
}

func TestPlaceForFill_DuplicateEventPlacesSingleStop(t *testing.T) {
	broker := models.NewMockBroker(1, nil)
	stopper := NewCompanionStopper(CompanionStopConfig{StopDistance: 5.0})

	fill := EntryFill{Symbol: "AAPL", EntrySide: models.TradierOrderSideBuy, Quantity: 10, FillPrice: 100.0, EntryOrderID: 11}

	placed, err := stopper.PlaceForFill(context.Background(), broker, models.ModePaper, fill)
	require.NoError(t, err)
	require.True(t, placed)

	// The redelivered fill event must not place a second stop.
	placed, err = stopper.PlaceForFill(context.Background(), broker, models.ModePaper, fill)
	require.NoError(t, err)
	require.False(t, placed)

	stops := broker.StopOrders()
	require.Len(t, stops, 1)
	require.Equal(t, CompanionStopTagForEntry(11), stops[0].Tag, "the stop carries the durable entry-order association in its tag")
}

func TestPlaceForFill_RestartWindowFindsDurableAssociation(t *testing.T) {
	broker := models.NewMockBroker(1, nil)

	first := NewCompanionStopper(CompanionStopConfig{StopDistance: 5.0})
	fill := EntryFill{Symbol: "AAPL", EntrySide: models.TradierOrderSideBuy, Quantity: 10, FillPrice: 100.0, EntryOrderID: 21}

	placed, err := first.PlaceForFill(context.Background(), broker, models.ModePaper, fill)
	require.NoError(t, err)
	require.True(t, placed)

	// Process restart: a FRESH stopper (empty in-memory set) replays the same
	// fill. The durable association — the tagged order at the broker — must
	// prevent a duplicate.
	restarted := NewCompanionStopper(CompanionStopConfig{StopDistance: 5.0})
	placed, err = restarted.PlaceForFill(context.Background(), broker, models.ModePaper, fill)
	require.NoError(t, err)
	require.False(t, placed)

	require.Len(t, broker.StopOrders(), 1, "restart replay must not double the protective size")
}

// failingStopBroker wraps MockBroker and fails every stop-order placement,
// simulating a broker rejection of the protective stop.
type failingStopBroker struct {
	*models.MockBroker
}

func (b *failingStopBroker) PlaceOrder(ctx context.Context, req *models.PlaceOrderRequest) (map[string]interface{}, error) {
	if req.OrderType == models.TradierOrderTypeStop {
		return nil, fmt.Errorf("broker rejected stop order")
	}
	return b.MockBroker.PlaceOrder(ctx, req)
}

func TestPlaceForFill_FailureIsLoudAndRecorded(t *testing.T) {
	telemetry.Init()
	broker := &failingStopBroker{MockBroker: models.NewMockBroker(1, nil)}
	stopper := NewCompanionStopper(CompanionStopConfig{StopDistance: 5.0})

	fill := EntryFill{Symbol: "TSLA", EntrySide: models.TradierOrderSideSellShort, Quantity: 3, FillPrice: 200.0, EntryOrderID: 31}

	placed, err := stopper.PlaceForFill(context.Background(), broker, models.ModeMargin, fill)
	require.Error(t, err)
	require.False(t, placed)

	// The failure counter incremented on the internal registry.
	var failures float64
	for _, p := range telemetry.Default.Snapshot() {
		if p.Name == "safety_companion_stop_failures_total" {
			failures += p.Value
		}
	}
	require.Equal(t, 1.0, failures)

	// The unprotected position is recorded for the alert engine, naming the
	// symbol and quantity.
	unprotected := stopper.UnprotectedPositions()
	require.Len(t, unprotected, 1)
	require.Equal(t, "TSLA", unprotected[0].Symbol)
	require.Equal(t, 3, unprotected[0].Quantity)
	require.Equal(t, uint(31), unprotected[0].EntryOrderID)
	require.Contains(t, unprotected[0].Reason, "broker rejected stop order")

	// The package-level provider serves the installed stopper's records.
	SetCompanionStopper(stopper)
	t.Cleanup(func() { SetCompanionStopper(nil) })
	require.Len(t, UnprotectedPositions(), 1)

	// A later successful placement for the same entry clears the record.
	okBroker := models.NewMockBroker(1, nil)
	placed, err = stopper.PlaceForFill(context.Background(), okBroker, models.ModeMargin, fill)
	require.NoError(t, err)
	require.True(t, placed)
	require.Empty(t, stopper.UnprotectedPositions())
}

func TestPlaceForFill_UnwiredProviderIsEmpty(t *testing.T) {
	SetCompanionStopper(nil)
	require.Empty(t, UnprotectedPositions(), "no stopper installed means no unprotected positions and a fully inert feature")
}
