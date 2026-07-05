package riskoverlay

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"

	models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
)

// snapshot builder that ignores the (nil) playground and returns a fixture.
func fixtureSnapshot(state PortfolioState, order ProposedOrder, scannedAt time.Time) PortfolioSnapshotFunc {
	return func(_ *models.Playground, _ *models.OrderRecord) (PortfolioState, ProposedOrder, time.Time, error) {
		return state, order, scannedAt, nil
	}
}

// errSnapshot is a snapshot builder that always fails, standing in for a
// transient DB/state error while building the portfolio snapshot.
func errSnapshot() PortfolioSnapshotFunc {
	return func(_ *models.Playground, _ *models.OrderRecord) (PortfolioState, ProposedOrder, time.Time, error) {
		return PortfolioState{}, ProposedOrder{}, time.Time{}, errors.New("boom: snapshot build failed")
	}
}

// erroringCrowdingLookup always fails ViewForScanCycle, standing in for a
// transient crowding-DB error.
type erroringCrowdingLookup struct{}

func (erroringCrowdingLookup) ViewForScanCycle(time.Time) (CrowdingView, error) {
	return CrowdingView{}, errors.New("boom: crowding lookup failed")
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

// B1 — a risk-reducing order is allowed even when BOTH the snapshot builder and
// the crowding lookup would error. The gate classifies the reduction side-first
// from the raw order and short-circuits ALLOW before any snapshot build or
// crowding lookup, so a transient DB error can never block a risk-reducing exit.
func TestGate_ReductionAllowedDespiteSnapshotAndLookupErrors(t *testing.T) {
	tight := RiskLimits{MaxGrossExposure: 1, MaxNetExposure: 1, MaxSectorConcentrationPct: 0, MaxDrawdownPct: 0, DeployableCapital: 0, RejectCrowdedEntries: true}
	// Both I/O seams error: if either were consulted before the reduction
	// short-circuit, the order would be blocked or the call would fail.
	gate := NewSimulationRiskGate(true, tight, erroringCrowdingLookup{}, errSnapshot())

	for _, side := range []models.TradierOrderSide{
		models.TradierOrderSideSell,
		models.TradierOrderSideSellToClose,
		models.TradierOrderSideBuyToClose,
		models.TradierOrderSideBuyToCover,
	} {
		t.Run(string(side), func(t *testing.T) {
			order := &models.OrderRecord{Symbol: "AAPL", Side: side, AbsoluteQuantity: 1_000_000, RequestedPrice: 1_000}
			require.NoError(t, gate.EvaluateSimulationOrder(nil, order))
		})
	}
}

// B1 — a NON-reducing order is permitted-with-warning when the crowding lookup
// errors: a crowding-DB hiccup must not become a trading halt (the kill switch
// owns halting, not this gate).
func TestGate_NonReductionPermissiveOnLookupError(t *testing.T) {
	limits := DefaultRiskLimits
	limits.RejectCrowdedEntries = true

	order := &models.OrderRecord{Symbol: "NVDA", Side: models.TradierOrderSideBuy, AbsoluteQuantity: 10, RequestedPrice: 100}
	scannedAt := time.Date(2026, 7, 4, 14, 0, 0, 0, time.UTC)
	gate := NewSimulationRiskGate(true, limits, erroringCrowdingLookup{}, fixtureSnapshot(PortfolioState{}, ProposedOrder{Ticker: "NVDA", SignedNotional: 1_000}, scannedAt))

	hook := logrustest.NewGlobal()
	defer hook.Reset()

	require.NoError(t, gate.EvaluateSimulationOrder(nil, order))

	var warned bool
	for _, e := range hook.AllEntries() {
		if e.Level == logrus.WarnLevel && strings.Contains(e.Message, "crowding view failed") {
			warned = true
		}
	}
	require.True(t, warned, "expected a loud Warn on fail-permissive crowding-lookup error")
}

// B1 — a NON-reducing order is permitted-with-warning when the snapshot builder
// errors, for the same reason.
func TestGate_NonReductionPermissiveOnSnapshotError(t *testing.T) {
	order := &models.OrderRecord{Symbol: "NVDA", Side: models.TradierOrderSideBuy, AbsoluteQuantity: 10, RequestedPrice: 100}
	gate := NewSimulationRiskGate(true, DefaultRiskLimits, erroringCrowdingLookup{}, errSnapshot())

	hook := logrustest.NewGlobal()
	defer hook.Reset()

	require.NoError(t, gate.EvaluateSimulationOrder(nil, order))

	var warned bool
	for _, e := range hook.AllEntries() {
		if e.Level == logrus.WarnLevel && strings.Contains(e.Message, "build snapshot failed") {
			warned = true
		}
	}
	require.True(t, warned, "expected a loud Warn on fail-permissive snapshot-build error")
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
