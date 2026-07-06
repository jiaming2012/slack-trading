package shadowdeploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSynthetic_HandComputedDivergence pins the fixture's documented
// expectations end to end through the engine, the divergence comparison, and
// the outcome comparison -- the same path the CLI's --synthetic mode prints.
func TestSynthetic_HandComputedDivergence(t *testing.T) {
	fixture := Synthetic()

	require.NoError(t, fixture.ActivePayload.Validate())
	require.NoError(t, fixture.ShadowPayload.Validate())
	require.Len(t, fixture.Observations, 10)

	activeDecisions := EvaluateConfig(fixture.ActivePayload, fixture.Observations)
	shadowDecisions := EvaluateConfig(fixture.ShadowPayload, fixture.Observations)

	selected := func(decisions []Decision) []string {
		out := []string{}
		for _, d := range decisions {
			if d.Selected {
				out = append(out, d.Ticker)
			}
		}
		return out
	}
	assert.ElementsMatch(t, []string{"AAPL", "AMD", "MSFT", "NVDA"}, selected(activeDecisions))
	assert.ElementsMatch(t, []string{"AAPL", "AMZN", "MSFT", "NVDA"}, selected(shadowDecisions))

	report := CompareDecisions(activeDecisions, shadowDecisions)
	assert.Equal(t, 10, report.TotalObservations)
	assert.Equal(t, 4, report.SelectedActive)
	assert.Equal(t, 4, report.SelectedShadow)
	assert.Equal(t, 3, report.SelectedBoth)
	require.Len(t, report.ShadowOnly, 1)
	assert.Equal(t, "AMZN", report.ShadowOnly[0].Ticker)
	require.Len(t, report.ActiveOnly, 1)
	assert.Equal(t, "AMD", report.ActiveOnly[0].Ticker)
	assert.InDelta(t, 40.0, report.DivergencePct, 1e-9, "2 divergent over a union of 5")

	outcomes := CompareOutcomes(activeDecisions, shadowDecisions, fixture.OutcomesByScanResult)

	assert.Equal(t, 4, outcomes.Active.Selections)
	assert.Equal(t, 4, outcomes.Active.WithOutcomes)
	assert.InDelta(t, 100.0, outcomes.Active.CoveragePct, 1e-9)
	assert.InDelta(t, 0.75, outcomes.Active.WinRate, 1e-9)
	assert.InDelta(t, 1.75, outcomes.Active.MeanPnlPct, 1e-9)

	assert.Equal(t, 4, outcomes.Shadow.Selections)
	assert.Equal(t, 3, outcomes.Shadow.WithOutcomes, "AMZN is deliberately uncovered")
	assert.InDelta(t, 75.0, outcomes.Shadow.CoveragePct, 1e-9)
	assert.InDelta(t, 2.0/3.0, outcomes.Shadow.WinRate, 1e-9)
	assert.InDelta(t, (2.5-1.0+4.0)/3.0, outcomes.Shadow.MeanPnlPct, 1e-9)

	// The no_model regime row stays visible on both sides.
	for _, decisions := range [][]Decision{activeDecisions, shadowDecisions} {
		var xom *Decision
		for i := range decisions {
			if decisions[i].Ticker == "XOM" {
				xom = &decisions[i]
			}
		}
		require.NotNil(t, xom)
		assert.True(t, xom.NoModel)
		assert.False(t, xom.Selected)
	}
}

// TestSynthetic_Deterministic: the fixture and everything derived from it is
// identical run to run (fixed ids, fixed clock, pure engine).
func TestSynthetic_Deterministic(t *testing.T) {
	first := Synthetic()
	second := Synthetic()
	assert.Equal(t, first, second)

	firstReport := CompareDecisions(
		EvaluateConfig(first.ActivePayload, first.Observations),
		EvaluateConfig(first.ShadowPayload, first.Observations),
	)
	secondReport := CompareDecisions(
		EvaluateConfig(second.ActivePayload, second.Observations),
		EvaluateConfig(second.ShadowPayload, second.Observations),
	)
	assert.Equal(t, firstReport, secondReport)
}
