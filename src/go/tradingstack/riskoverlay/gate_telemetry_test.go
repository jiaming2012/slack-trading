package riskoverlay

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

// metricValue sums the named series (optionally filtered to one label value)
// from the process-wide telemetry registry. found is false when no matching
// series exists at all.
func metricValue(t *testing.T, name string, labelKey, labelValue string) (float64, bool) {
	t.Helper()
	var total float64
	found := false
	for _, p := range telemetry.Default.Snapshot() {
		if p.Name != name {
			continue
		}
		if labelKey != "" && p.Labels[labelKey] != labelValue {
			continue
		}
		total += p.Value
		found = true
	}
	return total, found
}

func requireCounter(t *testing.T, name, labelKey, labelValue string, want float64) {
	t.Helper()
	got, _ := metricValue(t, name, labelKey, labelValue)
	require.Equal(t, want, got, "%s{%s=%s}", name, labelKey, labelValue)
}

func buyEntry(symbol string) *models.OrderRecord {
	return &models.OrderRecord{
		Class:            models.OrderRecordClassEquity,
		Symbol:           symbol,
		Side:             models.TradierOrderSideBuy,
		AbsoluteQuantity: 10,
		RequestedPrice:   100,
	}
}

// 5.2 — a snapshot error permits the order and counts degradation with
// reason snapshot_error.
func TestGateTelemetry_SnapshotErrorCountsDegradation(t *testing.T) {
	telemetry.Init()
	gate := NewSimulationRiskGate(true, DefaultRiskLimits, nil, errSnapshot())

	require.NoError(t, gate.EvaluateSimulationOrder(nil, buyEntry("NVDA")))
	requireCounter(t, telemetry.MetricRiskOverlayDegraded, "reason", DegradedReasonSnapshotError, 1)
}

// 5.2 — a crowding-lookup error permits the order and counts degradation with
// reason crowding_lookup_error.
func TestGateTelemetry_CrowdingErrorCountsDegradation(t *testing.T) {
	telemetry.Init()
	order := ProposedOrder{Ticker: "NVDA", Sector: "technology", SignedNotional: 1_000}
	gate := NewSimulationRiskGate(true, DefaultRiskLimits, erroringCrowdingLookup{}, fixtureSnapshot(PortfolioState{EvWeights: map[string]float64{"A": 1}}, order, time.Time{}))

	require.NoError(t, gate.EvaluateSimulationOrder(nil, buyEntry("NVDA")))
	requireCounter(t, telemetry.MetricRiskOverlayDegraded, "reason", DegradedReasonCrowdingLookupError, 1)
}

// 5.2 — an unknown-sector entry is still evaluated but counted as degradation
// with reason sector_unknown.
func TestGateTelemetry_UnknownSectorCountsDegradation(t *testing.T) {
	telemetry.Init()
	order := ProposedOrder{Ticker: "ZZZQ", Sector: "", StrategyID: "A", SignedNotional: 1_000}
	gate := NewSimulationRiskGate(true, DefaultRiskLimits, nil, fixtureSnapshot(PortfolioState{EvWeights: map[string]float64{"A": 1}}, order, time.Time{}))

	require.NoError(t, gate.EvaluateSimulationOrder(nil, buyEntry("ZZZQ")))
	requireCounter(t, telemetry.MetricRiskOverlayDegraded, "reason", DegradedReasonSectorUnknown, 1)
}

// 5.2 — a rejection increments the rejections counter once per breached limit
// family, labeled by limit_type.
func TestGateTelemetry_RejectionCountsPerBreachedFamily(t *testing.T) {
	telemetry.Init()
	limits := DefaultRiskLimits
	limits.MaxGrossExposure = 500
	limits.MaxNetExposure = 500

	order := ProposedOrder{Ticker: "NVDA", Sector: "technology", SignedNotional: 1_000}
	gate := NewSimulationRiskGate(true, limits, nil, fixtureSnapshot(PortfolioState{}, order, time.Time{}))

	err := gate.EvaluateSimulationOrder(nil, buyEntry("NVDA"))
	var rejected *RejectedError
	require.True(t, errors.As(err, &rejected))
	require.Len(t, rejected.Breaches, 2)

	requireCounter(t, telemetry.MetricRiskOverlayRejections, "limit_type", string(LimitGrossExposure), 1)
	requireCounter(t, telemetry.MetricRiskOverlayRejections, "limit_type", string(LimitNetExposure), 1)
}

// 5.2 — the ev_family_active gauge tracks each EV lookup result the gate
// evaluates with: 1 for a non-empty set, 0 for the pinned-inactive empty set.
func TestGateTelemetry_EvFamilyActiveGauge(t *testing.T) {
	telemetry.Init()

	nonEmpty := fixtureSnapshot(PortfolioState{EvWeights: map[string]float64{"A": 1}}, ProposedOrder{Ticker: "KO", Sector: "staples", StrategyID: "A", SignedNotional: 100}, time.Time{})
	gate := NewSimulationRiskGate(true, DefaultRiskLimits, nil, nonEmpty)
	require.NoError(t, gate.EvaluateSimulationOrder(nil, buyEntry("KO")))
	v, found := metricValue(t, telemetry.MetricRiskOverlayEvFamilyActive, "", "")
	require.True(t, found)
	require.Equal(t, 1.0, v)

	empty := fixtureSnapshot(PortfolioState{}, ProposedOrder{Ticker: "KO", Sector: "staples", SignedNotional: 100}, time.Time{})
	gate = NewSimulationRiskGate(true, DefaultRiskLimits, nil, empty)
	require.NoError(t, gate.EvaluateSimulationOrder(nil, buyEntry("KO")))
	v, found = metricValue(t, telemetry.MetricRiskOverlayEvFamilyActive, "", "")
	require.True(t, found)
	require.Equal(t, 0.0, v)
}

// 5.2 — a DISABLED gate is permissive-blind: it permits without evaluating and
// records NOTHING (no degradation, no rejections, no EV gauge).
func TestGateTelemetry_DisabledGateRecordsNothing(t *testing.T) {
	telemetry.Init()
	tight := RiskLimits{MaxGrossExposure: 1, MaxNetExposure: 1, MaxSectorConcentrationPct: 0, MaxDrawdownPct: 0, DeployableCapital: 0, RejectCrowdedEntries: true}
	gate := NewSimulationRiskGate(false, tight, erroringCrowdingLookup{}, errSnapshot())

	require.NoError(t, gate.EvaluateSimulationOrder(nil, buyEntry("NVDA")))

	_, found := metricValue(t, telemetry.MetricRiskOverlayDegraded, "", "")
	require.False(t, found, "disabled gate must record no degradation")
	_, found = metricValue(t, telemetry.MetricRiskOverlayRejections, "", "")
	require.False(t, found, "disabled gate must record no rejections")
	_, found = metricValue(t, telemetry.MetricRiskOverlayEvFamilyActive, "", "")
	require.False(t, found, "disabled gate must not touch the EV gauge")
}

// 4.3 / 5.2 — a reduction short-circuits before all I/O and records NO
// degradation even while every lookup and the snapshot builder are erroring.
func TestGateTelemetry_ReductionRecordsNoDegradation(t *testing.T) {
	telemetry.Init()
	gate := NewSimulationRiskGate(true, DefaultRiskLimits, erroringCrowdingLookup{}, errSnapshot())

	reduction := &models.OrderRecord{
		Class:            models.OrderRecordClassEquity,
		Symbol:           "AAPL",
		Side:             models.TradierOrderSideSellToClose,
		AbsoluteQuantity: 10,
		RequestedPrice:   100,
	}
	require.NoError(t, gate.EvaluateSimulationOrder(nil, reduction))

	_, found := metricValue(t, telemetry.MetricRiskOverlayDegraded, "", "")
	require.False(t, found, "a reduction must not be recorded as degradation")
}

// Adversarial review MAJOR 1 — a system-generated order bypasses the overlay
// BEFORE any snapshot build or lookup, and records nothing, even while every
// I/O seam is erroring and every limit is impossibly tight.
func TestGateTelemetry_SystemOrderBypassesBeforeIOAndRecordsNothing(t *testing.T) {
	telemetry.Init()
	tight := RiskLimits{MaxGrossExposure: 1, MaxNetExposure: 1, MaxSectorConcentrationPct: 0, MaxDrawdownPct: 0, DeployableCapital: 0, RejectCrowdedEntries: true}
	gate := NewSimulationRiskGate(true, tight, erroringCrowdingLookup{}, errSnapshot())

	settlement := &models.OrderRecord{
		Class:            models.OrderRecordClassEquity,
		Symbol:           "AAPL",
		Side:             models.TradierOrderSideBuy, // an entry by side classification alone
		AbsoluteQuantity: 100,
		RequestedPrice:   230,
		IsSystemOrder:    true,
	}
	require.NoError(t, gate.EvaluateSimulationOrder(nil, settlement))

	_, found := metricValue(t, telemetry.MetricRiskOverlayDegraded, "", "")
	require.False(t, found, "a system-order bypass must not be recorded as degradation")
	_, found = metricValue(t, telemetry.MetricRiskOverlayRejections, "", "")
	require.False(t, found, "a system order must never be rejected by the overlay")
}

// Adversarial review minor 3 — degradation is scoped to PERMITS: a REJECTED
// unknown-sector order does not increment sector_unknown.
func TestGateTelemetry_RejectedUnknownSectorNotCountedAsDegradation(t *testing.T) {
	telemetry.Init()
	limits := DefaultRiskLimits
	limits.MaxGrossExposure = 500 // the 1_000 notional entry breaches

	order := ProposedOrder{Ticker: "ZZZQ", Sector: "", StrategyID: "A", SignedNotional: 1_000}
	gate := NewSimulationRiskGate(true, limits, nil, fixtureSnapshot(PortfolioState{EvWeights: map[string]float64{"A": 1}}, order, time.Time{}))

	err := gate.EvaluateSimulationOrder(nil, buyEntry("ZZZQ"))
	var rejected *RejectedError
	require.True(t, errors.As(err, &rejected))

	got, _ := metricValue(t, telemetry.MetricRiskOverlayDegraded, "reason", DegradedReasonSectorUnknown)
	require.Equal(t, 0.0, got, "a rejected order must not count sector_unknown degradation")
	requireCounter(t, telemetry.MetricRiskOverlayRejections, "limit_type", string(LimitGrossExposure), 1)
}
