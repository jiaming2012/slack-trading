package riskoverlay

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	coremodels "github.com/jiaming2012/slack-trading/src/go/models"
)

// newSnapshotFixture builds a real Simulation playground holding:
//   - AAPL equity long 100 @ 150 (tag "cc-v7"), feed price 155 (+500 PL)
//   - KO equity long 50 @ 60 (tag "wheel"), feed price 60
//   - AAPL 2025-09-05 C230 short 2 contracts @ 2.50 (empty tag -> client ID)
//   - XLE equity long 10 @ 80 with NO current price (cost-basis fallback),
//     injected directly into the position cache
//   - a 7-session equity plot (with an intraday double-point on one session)
func newSnapshotFixture(t *testing.T) *models.Playground {
	t.Helper()

	clientID := "client-1"
	start := time.Date(2025, time.September, 3, 13, 30, 0, 0, time.UTC)
	end := start.Add(48 * time.Hour)
	period := time.Minute
	source := coremodels.CandleRepositorySource{Type: "test"}

	makeFeed := func(sym coremodels.Instrument, close float64) *models.CandleRepository {
		var candles []*coremodels.PolygonAggregateBarV2
		for i := 0; i < 30; i++ {
			candles = append(candles, &coremodels.PolygonAggregateBarV2{
				Timestamp: start.Add(time.Duration(i) * period),
				Close:     close,
			})
		}
		repo, err := models.NewCandleRepository(sym, period, candles, []string{}, nil, 0, source)
		require.NoError(t, err)
		return repo
	}

	feeds := []*models.CandleRepository{
		makeFeed(coremodels.StockSymbol("AAPL"), 155),
		makeFeed(coremodels.StockSymbol("KO"), 60),
		makeFeed(coremodels.OptionSymbol("AAPL250905C230000"), 2.50),
	}

	orders := []*models.OrderRecord{
		{
			Class:            models.OrderRecordClassEquity,
			Symbol:           "AAPL",
			Side:             models.TradierOrderSideBuyToOpen,
			AbsoluteQuantity: 100,
			RequestedPrice:   150,
			Status:           models.OrderRecordStatusFilled,
			Tag:              "cc-v7",
			Trades:           []*models.TradeRecord{{Quantity: 100, Price: 150, Timestamp: start}},
		},
		{
			Class:            models.OrderRecordClassEquity,
			Symbol:           "KO",
			Side:             models.TradierOrderSideBuyToOpen,
			AbsoluteQuantity: 50,
			RequestedPrice:   60,
			Status:           models.OrderRecordStatusFilled,
			Tag:              "wheel",
			Trades:           []*models.TradeRecord{{Quantity: 50, Price: 60, Timestamp: start}},
		},
		{
			Class:            models.OrderRecordClassOption,
			Symbol:           "AAPL250905C230000",
			Side:             models.TradierOrderSideSellToOpen,
			AbsoluteQuantity: 2,
			RequestedPrice:   2.50,
			Status:           models.OrderRecordStatusFilled,
			Tag:              "", // falls back to the playground client ID
			Trades:           []*models.TradeRecord{{Quantity: -2, Price: 2.50, Timestamp: start}},
		},
	}

	p, err := models.NewPlayground(models.PlaygroundConfig{
		ClientID:       &clientID,
		Balance:        100_000,
		Mode:           models.ModeSimulation,
		Clock:          models.NewClock(start, end, nil),
		Now:            start,
		BackfillOrders: orders,
		Feeds:          feeds,
	})
	require.NoError(t, err)

	// A position with NO current price and no feed, injected directly into the
	// cache: prices through the cost-basis fallback and resolves no sector.
	p.GetPositionCache().Set(coremodels.NewStockSymbol("XLE"), &models.Position{Quantity: 10, CostBasis: 80})

	// Session dates cohere with the sim clock: the clock sits inside the
	// 2025-09-03 session, and the plot carries a stale intraday point for that
	// CURRENT session (so the builder's replace-current-session path applies).
	p.SetEquityPlot(fixtureEquityPlot(true))

	return p
}

// fixtureEquityPlot builds the fixture's equity plot; includeCurrentSession
// controls whether the CURRENT session (2025-09-03, the sim clock's date)
// carries a stale intraday point.
func fixtureEquityPlot(includeCurrentSession bool) []*coremodels.EquityPlot {
	day := func(month time.Month, d, hour int) time.Time {
		return time.Date(2025, month, d, hour, 0, 0, 0, time.UTC)
	}
	plot := []*coremodels.EquityPlot{
		{Timestamp: day(time.August, 26, 20), Value: 100_000},
		{Timestamp: day(time.August, 27, 20), Value: 100_100},
		{Timestamp: day(time.August, 28, 15), Value: 100_150}, // intraday point...
		{Timestamp: day(time.August, 28, 20), Value: 100_200}, // ...same session's close wins
		{Timestamp: day(time.August, 29, 20), Value: 100_300},
		{Timestamp: day(time.September, 1, 20), Value: 100_400},
		{Timestamp: day(time.September, 2, 20), Value: 100_450},
	}
	if includeCurrentSession {
		plot = append(plot, &coremodels.EquityPlot{Timestamp: day(time.September, 3, 12), Value: 99_999}) // current session's stale point, replaced by current equity
	}
	return plot
}

func findPosition(t *testing.T, positions []LogicalPosition, ticker string) LogicalPosition {
	t.Helper()
	for _, p := range positions {
		if p.Ticker == ticker {
			return p
		}
	}
	t.Fatalf("position %q not found in %+v", ticker, positions)
	return LogicalPosition{}
}

// 4.2 — the seeded playground fixture yields the expected PortfolioState and
// completed ProposedOrder, end to end with fakes and no database.
func TestBuildPortfolioSnapshot_SeededPlayground(t *testing.T) {
	p := newSnapshotFixture(t)

	evFake := NewFakeEvWeightLookup(map[string]float64{"cc-v7": 0.75, "wheel": 0.25})
	secFake := NewFakeSectorLookup(map[string]string{"AAPL": "technology", "KO": "staples", "MSFT": "technology"})
	snapshot := BuildPortfolioSnapshot(evFake, secFake)

	order := &models.OrderRecord{
		Class:            models.OrderRecordClassEquity,
		Symbol:           "MSFT",
		Side:             models.TradierOrderSideBuyToOpen,
		AbsoluteQuantity: 10,
		RequestedPrice:   400,
		Tag:              "cc-v7",
	}

	state, proposed, scannedAt, err := snapshot(p, order)
	require.NoError(t, err)
	require.False(t, scannedAt.IsZero())

	// Positions: signed notionals with current-price/cost-basis pricing and the
	// x100 option multiplier; option sector resolved via the UNDERLYING.
	require.Len(t, state.Positions, 4)
	aapl := findPosition(t, state.Positions, "AAPL")
	require.Equal(t, 15_500.0, aapl.SignedNotional, "100 x 155 current price")
	require.Equal(t, "technology", aapl.Sector)

	ko := findPosition(t, state.Positions, "KO")
	require.Equal(t, 3_000.0, ko.SignedNotional, "50 x 60")
	require.Equal(t, "staples", ko.Sector)

	opt := findPosition(t, state.Positions, "AAPL250905C230000")
	require.Equal(t, -500.0, opt.SignedNotional, "-2 x 2.50 x 100 contract multiplier")
	require.Equal(t, "technology", opt.Sector, "option sector resolves via the underlying ticker")

	xle := findPosition(t, state.Positions, "XLE")
	require.Equal(t, 800.0, xle.SignedNotional, "10 x 80 cost-basis fallback (no current price)")
	require.Equal(t, "", xle.Sector, "unknown position ticker resolves to the empty sector")

	// Strategy deployed capital, attributed by tag with client-ID fallback.
	require.Equal(t, map[string]float64{
		"cc-v7":    15_500.0,
		"wheel":    3_000.0,
		"client-1": 500.0,
	}, state.StrategyDeployed)

	// EV weights pass through the seam.
	require.Equal(t, map[string]float64{"cc-v7": 0.75, "wheel": 0.25}, state.EvWeights)

	// Equity series: exactly the last 5 sessions' closes (intraday point
	// collapsed away, older sessions dropped), ending with CURRENT equity
	// (100_000 balance + 500 unrealized PL), never the stale close.
	require.Equal(t, []float64{100_200, 100_300, 100_400, 100_450, 100_500}, state.EquitySeries)

	// The proposed order is completed with sector + strategy identity.
	require.Equal(t, "MSFT", proposed.Ticker)
	require.Equal(t, 4_000.0, proposed.SignedNotional)
	require.Equal(t, "technology", proposed.Sector)
	require.Equal(t, "cc-v7", proposed.StrategyID)
	require.False(t, proposed.IsReduction)
}

// 4.2 — an order into a ticker with no known sector is evaluated with the
// empty sector (exempt from the sector-concentration family), not an error.
func TestBuildPortfolioSnapshot_UnknownSectorOrder(t *testing.T) {
	p := newSnapshotFixture(t)

	evFake := NewFakeEvWeightLookup(map[string]float64{"cc-v7": 1.0})
	secFake := NewFakeSectorLookup(map[string]string{"AAPL": "technology", "KO": "staples"})
	snapshot := BuildPortfolioSnapshot(evFake, secFake)

	order := &models.OrderRecord{
		Class:            models.OrderRecordClassEquity,
		Symbol:           "ZZZQ",
		Side:             models.TradierOrderSideBuyToOpen,
		AbsoluteQuantity: 10,
		RequestedPrice:   5,
		Tag:              "cc-v7",
	}

	_, proposed, _, err := snapshot(p, order)
	require.NoError(t, err)
	require.Equal(t, "", proposed.Sector)
}

// An empty-tag order attributes to the playground client ID, mirroring the
// deployed-capital attribution.
func TestBuildPortfolioSnapshot_EmptyTagFallsBackToClientID(t *testing.T) {
	p := newSnapshotFixture(t)
	snapshot := BuildPortfolioSnapshot(NewFakeEvWeightLookup(nil), NewFakeSectorLookup(nil))

	order := &models.OrderRecord{
		Class:            models.OrderRecordClassEquity,
		Symbol:           "MSFT",
		Side:             models.TradierOrderSideBuyToOpen,
		AbsoluteQuantity: 1,
		RequestedPrice:   400,
	}
	_, proposed, _, err := snapshot(p, order)
	require.NoError(t, err)
	require.Equal(t, "client-1", proposed.StrategyID)
}

// An EV-weight or sector lookup failure surfaces as a snapshot error (the gate
// then permits fail-permissive); a nil playground errors instead of panicking.
func TestBuildPortfolioSnapshot_LookupErrorsSurfaceAsSnapshotErrors(t *testing.T) {
	p := newSnapshotFixture(t)
	order := &models.OrderRecord{
		Class:            models.OrderRecordClassEquity,
		Symbol:           "MSFT",
		Side:             models.TradierOrderSideBuyToOpen,
		AbsoluteQuantity: 1,
		RequestedPrice:   400,
	}

	t.Run("EV lookup error", func(t *testing.T) {
		ev := NewFakeEvWeightLookup(nil)
		ev.Err = errors.New("db down")
		_, _, _, err := BuildPortfolioSnapshot(ev, NewFakeSectorLookup(nil))(p, order)
		require.Error(t, err)
	})

	t.Run("sector lookup error", func(t *testing.T) {
		sec := NewFakeSectorLookup(nil)
		sec.Err = errors.New("db down")
		_, _, _, err := BuildPortfolioSnapshot(NewFakeEvWeightLookup(nil), sec)(p, order)
		require.Error(t, err)
	})

	t.Run("nil playground errors without panicking", func(t *testing.T) {
		_, _, _, err := BuildPortfolioSnapshot(NewFakeEvWeightLookup(nil), NewFakeSectorLookup(nil))(nil, order)
		require.Error(t, err)
	})

	t.Run("malformed playground degrades to an error, not a crash", func(t *testing.T) {
		_, _, _, err := BuildPortfolioSnapshot(NewFakeEvWeightLookup(nil), NewFakeSectorLookup(nil))(&models.Playground{}, order)
		require.Error(t, err)
	})
}

// 4.2 — end-to-end gate decision over the real snapshot builder with fakes and
// no database: a tightened limit rejects, permissive defaults permit.
func TestGate_EndToEndWithSnapshotBuilderNoDatabase(t *testing.T) {
	p := newSnapshotFixture(t)

	evFake := NewFakeEvWeightLookup(map[string]float64{"cc-v7": 0.75, "wheel": 0.25})
	secFake := NewFakeSectorLookup(map[string]string{"AAPL": "technology", "KO": "staples", "MSFT": "technology"})
	snapshot := BuildPortfolioSnapshot(evFake, secFake)

	entry := &models.OrderRecord{
		Class:            models.OrderRecordClassEquity,
		Symbol:           "MSFT",
		Side:             models.TradierOrderSideBuyToOpen,
		AbsoluteQuantity: 100,
		RequestedPrice:   400, // 40k notional on top of 19k gross
		Tag:              "cc-v7",
	}

	t.Run("tightened gross-exposure limit rejects with the full breach list", func(t *testing.T) {
		limits := DefaultRiskLimits
		limits.MaxGrossExposure = 20_000
		gate := NewSimulationRiskGate(true, limits, NewFakeCrowdingLookup(), snapshot)

		err := gate.EvaluateSimulationOrder(p, entry)
		require.Error(t, err)
		var rejected *RejectedError
		require.True(t, errors.As(err, &rejected))
		require.Equal(t, LimitGrossExposure, rejected.Breaches[0].Type)
	})

	t.Run("permissive defaults permit the same order", func(t *testing.T) {
		gate := NewSimulationRiskGate(true, DefaultRiskLimits, NewFakeCrowdingLookup(), snapshot)
		require.NoError(t, gate.EvaluateSimulationOrder(p, entry))
	})
}

// 4.3 — the reduction-bypass-before-I/O invariant, re-asserted against the
// REAL snapshot builder: with the snapshot builder's lookups AND the crowding
// lookup all erroring, a sell-to-close order is permitted and NO lookup is
// consulted (design D2 / N1: placeOrder re-derives closeable volume, so a
// side-classified reduction can only reduce).
func TestGate_ReductionBypassesRealSnapshotBuilderBeforeAnyIO(t *testing.T) {
	p := newSnapshotFixture(t)

	evFake := NewFakeEvWeightLookup(nil)
	evFake.Err = errors.New("ev db down")
	secFake := NewFakeSectorLookup(nil)
	secFake.Err = errors.New("sector db down")

	tight := RiskLimits{MaxGrossExposure: 1, MaxNetExposure: 1, MaxSectorConcentrationPct: 0, MaxDrawdownPct: 0, DeployableCapital: 0, RejectCrowdedEntries: true}
	gate := NewSimulationRiskGate(true, tight, erroringCrowdingLookup{}, BuildPortfolioSnapshot(evFake, secFake))

	reduction := &models.OrderRecord{
		Class:            models.OrderRecordClassEquity,
		Symbol:           "AAPL",
		Side:             models.TradierOrderSideSellToClose,
		AbsoluteQuantity: 100,
		RequestedPrice:   155,
		Tag:              "cc-v7",
	}

	require.NoError(t, gate.EvaluateSimulationOrder(p, reduction))
	require.Equal(t, 0, evFake.Calls, "EV lookup must never be consulted for a reduction")
	require.Equal(t, 0, secFake.Calls, "sector lookup must never be consulted for a reduction")
}

// Adversarial review minor 5 — when the CURRENT session has no plot point yet,
// current equity is APPENDED as a new session instead of overwriting the prior
// session's close (overwriting would shorten the window and could understate
// the peak).
func TestBuildPortfolioSnapshot_EquitySeriesAppendsWhenCurrentSessionHasNoPoint(t *testing.T) {
	p := newSnapshotFixture(t)
	p.SetEquityPlot(fixtureEquityPlot(false)) // plot ends 2025-09-02; the clock's session (09-03) has no point

	snapshot := BuildPortfolioSnapshot(
		NewFakeEvWeightLookup(map[string]float64{"cc-v7": 1}),
		NewFakeSectorLookup(map[string]string{"MSFT": "technology"}),
	)
	order := buyEntry("MSFT")
	order.Tag = "cc-v7"

	state, _, _, err := snapshot(p, order)
	require.NoError(t, err)

	// Prior sessions' closes are PRESERVED (100_450 is 09-02's close, not
	// overwritten) and current equity (100_000 + 500 PL) is its own new point.
	require.Equal(t, []float64{100_200, 100_300, 100_400, 100_450, 100_500}, state.EquitySeries)
}
