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

	fill := EntryFill{Symbol: "AAPL", EntrySide: models.TradierOrderSideBuy, FillPrice: 100.0, EntryOrderID: 11, TotalFilledQuantity: 10}

	placed, err := stopper.PlaceForFill(context.Background(), broker, models.ModePaper, fill)
	require.NoError(t, err)
	require.True(t, placed)

	// The redelivered fill event carries no NEW filled quantity: delta 0, so
	// no second stop.
	placed, err = stopper.PlaceForFill(context.Background(), broker, models.ModePaper, fill)
	require.NoError(t, err)
	require.False(t, placed)

	stops := broker.StopOrders()
	require.Len(t, stops, 1)
	require.Equal(t, 10, stops[0].Quantities[0])
	require.Equal(t, CompanionStopTagForEntry(11), stops[0].Tag, "the stop carries the durable entry-order association in its tag")
}

// One strategy order netting into multiple trades: every further trade for an
// already-protected entry places a TOP-UP stop for its delta, so total stop
// quantity equals total filled quantity — never the requested total, and
// never a silently skipped remainder.
func TestPlaceForFill_MultiTradeFillToppedUpToTotalFilled(t *testing.T) {
	broker := models.NewMockBroker(1, nil)
	stopper := NewCompanionStopper(CompanionStopConfig{StopDistance: 5.0})

	// Trade 1: 4 of 10 filled.
	placed, err := stopper.PlaceForFill(context.Background(), broker, models.ModePaper, EntryFill{
		Symbol: "AAPL", EntrySide: models.TradierOrderSideBuy, FillPrice: 100.0, EntryOrderID: 11, TotalFilledQuantity: 4,
	})
	require.NoError(t, err)
	require.True(t, placed)

	// Trade 2: cumulative 10 filled — a top-up stop for the delta of 6.
	placed, err = stopper.PlaceForFill(context.Background(), broker, models.ModePaper, EntryFill{
		Symbol: "AAPL", EntrySide: models.TradierOrderSideBuy, FillPrice: 101.0, EntryOrderID: 11, TotalFilledQuantity: 10,
	})
	require.NoError(t, err)
	require.True(t, placed)

	stops := broker.StopOrders()
	require.Len(t, stops, 2)
	total := 0
	for _, s := range stops {
		require.Equal(t, CompanionStopTagForEntry(11), s.Tag)
		total += s.Quantities[0]
	}
	require.Equal(t, 10, total, "total stop quantity must equal total filled quantity")
	require.Equal(t, 10.0, stopper.ProtectedQuantity(11))
}

// A top-up placement failure must be loud for the DELTA left unprotected, and
// the already-placed coverage must remain tracked so a retry sizes only the
// missing remainder.
func TestPlaceForFill_TopUpFailureAlertsForTheDelta(t *testing.T) {
	telemetry.Init()
	mock := models.NewMockBroker(1, nil)
	stopper := NewCompanionStopper(CompanionStopConfig{StopDistance: 5.0})

	placed, err := stopper.PlaceForFill(context.Background(), mock, models.ModePaper, EntryFill{
		Symbol: "AAPL", EntrySide: models.TradierOrderSideBuy, FillPrice: 100.0, EntryOrderID: 11, TotalFilledQuantity: 4,
	})
	require.NoError(t, err)
	require.True(t, placed)

	// The broker rejects the top-up for the second trade's delta.
	failing := &failingStopBroker{MockBroker: mock}
	placed, err = stopper.PlaceForFill(context.Background(), failing, models.ModePaper, EntryFill{
		Symbol: "AAPL", EntrySide: models.TradierOrderSideBuy, FillPrice: 101.0, EntryOrderID: 11, TotalFilledQuantity: 10,
	})
	require.Error(t, err)
	require.False(t, placed)

	unprotected := stopper.UnprotectedPositions()
	require.Len(t, unprotected, 1)
	require.Equal(t, 6, unprotected[0].Quantity, "the alert must name the unprotected DELTA, not the whole order")
	require.Equal(t, 4.0, stopper.ProtectedQuantity(11), "existing coverage stays tracked")

	// A retry through a healthy broker covers exactly the missing 6.
	placed, err = stopper.PlaceForFill(context.Background(), mock, models.ModePaper, EntryFill{
		Symbol: "AAPL", EntrySide: models.TradierOrderSideBuy, FillPrice: 101.0, EntryOrderID: 11, TotalFilledQuantity: 10,
	})
	require.NoError(t, err)
	require.True(t, placed)
	stops := mock.StopOrders()
	require.Len(t, stops, 2)
	require.Equal(t, 6, stops[1].Quantities[0])
	require.Empty(t, stopper.UnprotectedPositions())
}

// Fractional remainders below one share skip with a Warn (not the
// failure/alert loop) and stay tracked, so accumulation across fills still
// protects once a whole share is outstanding.
func TestPlaceForFill_FractionalRemainderSkippedAndAccumulated(t *testing.T) {
	telemetry.Init()
	broker := models.NewMockBroker(1, nil)
	stopper := NewCompanionStopper(CompanionStopConfig{StopDistance: 5.0})

	// 0.6 shares filled: below one whole share — no stop, no failure record.
	placed, err := stopper.PlaceForFill(context.Background(), broker, models.ModePaper, EntryFill{
		Symbol: "AAPL", EntrySide: models.TradierOrderSideBuy, FillPrice: 100.0, EntryOrderID: 11, TotalFilledQuantity: 0.6,
	})
	require.NoError(t, err)
	require.False(t, placed)
	require.Empty(t, broker.StopOrders())
	require.Empty(t, stopper.UnprotectedPositions(), "a fractional skip must not enter the failure/alert loop")

	// Accumulation reaches 1.2 shares: one whole share gets protected; the
	// residual 0.2 stays outstanding in the tracker.
	placed, err = stopper.PlaceForFill(context.Background(), broker, models.ModePaper, EntryFill{
		Symbol: "AAPL", EntrySide: models.TradierOrderSideBuy, FillPrice: 100.0, EntryOrderID: 11, TotalFilledQuantity: 1.2,
	})
	require.NoError(t, err)
	require.True(t, placed)
	stops := broker.StopOrders()
	require.Len(t, stops, 1)
	require.Equal(t, 1, stops[0].Quantities[0])
	require.Equal(t, 1.0, stopper.ProtectedQuantity(11))
}

func TestPlaceForFill_RestartWindowFindsDurableAssociation(t *testing.T) {
	broker := models.NewMockBroker(1, nil)

	first := NewCompanionStopper(CompanionStopConfig{StopDistance: 5.0})
	fill := EntryFill{Symbol: "AAPL", EntrySide: models.TradierOrderSideBuy, FillPrice: 100.0, EntryOrderID: 21, TotalFilledQuantity: 10}

	placed, err := first.PlaceForFill(context.Background(), broker, models.ModePaper, fill)
	require.NoError(t, err)
	require.True(t, placed)

	// Process restart: a FRESH stopper (empty in-memory tracker) replays the
	// same fill. The durable association — the tagged stop order at the
	// broker — must prevent a duplicate.
	restarted := NewCompanionStopper(CompanionStopConfig{StopDistance: 5.0})
	placed, err = restarted.PlaceForFill(context.Background(), broker, models.ModePaper, fill)
	require.NoError(t, err)
	require.False(t, placed)

	require.Len(t, broker.StopOrders(), 1, "restart replay must not double the protective size")
	require.Equal(t, 10.0, restarted.ProtectedQuantity(21), "the durable scan seeds the cumulative baseline")
}

// The durable scan requires the FULL match — tag AND stop order type AND
// symbol. A same-tag order of the wrong type or symbol must not count as
// protection (a poisoned or unrelated order suppressing a real stop is worse
// than a duplicate protective stop).
func TestPlaceForFill_DurableScanRequiresTagTypeAndSymbol(t *testing.T) {
	broker := models.NewMockBroker(1, nil)

	// Seed the broker with two decoys carrying the entry's tag: a MARKET
	// order (wrong type) and a stop for a DIFFERENT symbol.
	stopAt := 95.0
	_, err := broker.PlaceOrder(context.Background(), &models.PlaceOrderRequest{
		Symbol:     "AAPL",
		Quantities: []int{10},
		Sides:      []models.TradierOrderSide{models.TradierOrderSideSell},
		OrderType:  models.TradierOrderTypeMarket,
		Class:      models.OrderRecordClassEquity,
		Tag:        CompanionStopTagForEntry(21),
	})
	require.NoError(t, err)
	_, err = broker.PlaceOrder(context.Background(), &models.PlaceOrderRequest{
		Symbol:     "TSLA",
		Quantities: []int{10},
		Sides:      []models.TradierOrderSide{models.TradierOrderSideSell},
		OrderType:  models.TradierOrderTypeStop,
		Class:      models.OrderRecordClassEquity,
		Tag:        CompanionStopTagForEntry(21),
		StopPrice:  &stopAt,
	})
	require.NoError(t, err)

	// A fresh stopper must NOT treat the decoys as protection: the real stop
	// for AAPL entry 21 still gets placed.
	stopper := NewCompanionStopper(CompanionStopConfig{StopDistance: 5.0})
	placed, err := stopper.PlaceForFill(context.Background(), broker, models.ModePaper, EntryFill{
		Symbol: "AAPL", EntrySide: models.TradierOrderSideBuy, FillPrice: 100.0, EntryOrderID: 21, TotalFilledQuantity: 10,
	})
	require.NoError(t, err)
	require.True(t, placed, "decoy orders matching only the tag must not suppress the protective stop")

	var aaplStops int
	for _, s := range broker.StopOrders() {
		if s.Symbol == "AAPL" {
			aaplStops++
			require.Equal(t, 10, s.Quantities[0])
		}
	}
	require.Equal(t, 1, aaplStops)
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

	fill := EntryFill{Symbol: "TSLA", EntrySide: models.TradierOrderSideSellShort, FillPrice: 200.0, EntryOrderID: 31, TotalFilledQuantity: 3}

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
