package riskoverlay

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
)

// snapshot builder that ignores the (nil) playground and returns a fixture.
func fixtureSnapshot(state PortfolioState, order ProposedOrder, scannedAt time.Time) PortfolioSnapshotFunc {
	return func(_ *models.Playground, _ *models.OrderRecord) (PortfolioState, ProposedOrder, time.Time, error) {
		return state, order, scannedAt, nil
	}
}

// A disabled gate is inert: it permits an order that would breach every limit.
func TestGate_DisabledIsInert(t *testing.T) {
	tight := RiskLimits{MaxGrossExposure: 1, MaxNetExposure: 1, MaxSectorConcentrationPct: 0, MaxDrawdownPct: 0, DeployableCapital: 0, RejectCrowdedEntries: true}
	order := ProposedOrder{Ticker: "NVDA", Sector: "technology", StrategyID: "X", SignedNotional: 1e9}
	gate := NewSimulationRiskGate(false, tight, nil, fixtureSnapshot(PortfolioState{}, order, time.Time{}))

	require.False(t, gate.Enabled())
	require.NoError(t, gate.EvaluateSimulationOrder(nil, nil))
}

// An enabled gate with no snapshot builder cannot construct engine inputs and
// therefore permits (fails open on missing wiring, never blocks trading).
func TestGate_EnabledNilSnapshotPermits(t *testing.T) {
	gate := NewSimulationRiskGate(true, DefaultRiskLimits, nil, nil)
	require.NoError(t, gate.EvaluateSimulationOrder(nil, nil))
}

// An enabled gate rejects a breaching order with a *RejectedError carrying the breaches.
func TestGate_EnabledRejectsBreachingOrder(t *testing.T) {
	limits := DefaultRiskLimits
	limits.MaxGrossExposure = 100_000
	state := PortfolioState{Positions: []LogicalPosition{{Ticker: "AAPL", SignedNotional: 95_000}}}
	order := ProposedOrder{Ticker: "MSFT", SignedNotional: 20_000} // gross -> 115k

	gate := NewSimulationRiskGate(true, limits, nil, fixtureSnapshot(state, order, time.Time{}))
	err := gate.EvaluateSimulationOrder(nil, nil)
	require.Error(t, err)

	var rejected *RejectedError
	require.True(t, errors.As(err, &rejected))
	require.NotEmpty(t, rejected.Breaches)
	require.Equal(t, LimitGrossExposure, rejected.Breaches[0].Type)
}

// An enabled gate permits an allowed order.
func TestGate_EnabledPermitsCleanOrder(t *testing.T) {
	gate := NewSimulationRiskGate(true, DefaultRiskLimits, nil, fixtureSnapshot(PortfolioState{}, ProposedOrder{Ticker: "KO", SignedNotional: 1_000}, time.Time{}))
	require.NoError(t, gate.EvaluateSimulationOrder(nil, nil))
}

// The gate consults the crowding lookup for the snapshot's scan cycle and
// rejects an entry into a flagged ticker.
func TestGate_ConsultsCrowdingLookup(t *testing.T) {
	scannedAt := time.Date(2026, 7, 4, 14, 0, 0, 0, time.UTC)
	lookup := NewFakeCrowdingLookup()
	lookup.Seed(scannedAt, true, []string{"NVDA"})

	limits := DefaultRiskLimits
	limits.RejectCrowdedEntries = true
	order := ProposedOrder{Ticker: "NVDA", SignedNotional: 1_000}

	gate := NewSimulationRiskGate(true, limits, lookup, fixtureSnapshot(PortfolioState{}, order, scannedAt))
	err := gate.EvaluateSimulationOrder(nil, nil)
	require.Error(t, err)
	var rejected *RejectedError
	require.True(t, errors.As(err, &rejected))
	require.Equal(t, LimitCrowding, rejected.Breaches[0].Type)
}

// MapProposedOrder classifies sides and computes signed notional.
func TestMapProposedOrder_Classification(t *testing.T) {
	px := 10.0
	cases := []struct {
		name      string
		side      models.TradierOrderSide
		wantSign  float64
		reduction bool
	}{
		{"buy opens long", models.TradierOrderSideBuy, +1000, false},
		{"buy_to_open opens long", models.TradierOrderSideBuyToOpen, +1000, false},
		{"sell_short opens short", models.TradierOrderSideSellShort, -1000, false},
		{"sell_to_open opens short", models.TradierOrderSideSellToOpen, -1000, false},
		{"sell reduces long", models.TradierOrderSideSell, -1000, true},
		{"sell_to_close reduces long", models.TradierOrderSideSellToClose, -1000, true},
		{"buy_to_close reduces short", models.TradierOrderSideBuyToClose, +1000, true},
		{"buy_to_cover reduces short", models.TradierOrderSideBuyToCover, +1000, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			order := &models.OrderRecord{Symbol: "AAPL", Side: tc.side, AbsoluteQuantity: 100, RequestedPrice: px}
			p := MapProposedOrder(order)
			require.Equal(t, "AAPL", p.Ticker)
			require.Equal(t, tc.wantSign, p.SignedNotional)
			require.Equal(t, tc.reduction, p.IsReduction)
		})
	}
}
