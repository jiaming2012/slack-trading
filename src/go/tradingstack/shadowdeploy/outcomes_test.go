package shadowdeploy

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// outcomeRow builds a sim_outcomes row with the given pnl (nil allowed).
func outcomeRow(scanResultID uuid.UUID, pnl *float64) tradingstack.SimOutcome {
	return tradingstack.SimOutcome{
		BaseModel:    tradingstack.BaseModel{ID: uuid.New()},
		ScanResultID: scanResultID,
		SimulatedAt:  time.Date(2026, 3, 3, 9, 30, 0, 0, time.UTC),
		PnlPct:       pnl,
		ExitReason:   "target",
	}
}

// TestSummarizeOutcomes_SeededFixture pins the spec fixture: 4 selections, 3
// with outcome rows (2 wins, 1 loss) -> coverage 75%, decided 3, win rate
// 2/3, mean pnl over the 3 covered rows.
func TestSummarizeOutcomes_SeededFixture(t *testing.T) {
	selections := []Decision{
		decisionFor("A", 0.9, true),
		decisionFor("B", 0.8, true),
		decisionFor("C", 0.7, true),
		decisionFor("D", 0.6, true),
		decisionFor("E", 0.1, false), // not selected: must not count anywhere
	}

	outcomes := map[uuid.UUID][]tradingstack.SimOutcome{
		selections[0].ScanResultID: {outcomeRow(selections[0].ScanResultID, floatPtr(2.0))},
		selections[1].ScanResultID: {outcomeRow(selections[1].ScanResultID, floatPtr(1.0))},
		selections[2].ScanResultID: {outcomeRow(selections[2].ScanResultID, floatPtr(-1.5))},
		// D: selected but uncovered.
		selections[4].ScanResultID: {outcomeRow(selections[4].ScanResultID, floatPtr(9.9))},
	}

	summary := SummarizeOutcomes("shadow", selections, outcomes)

	assert.Equal(t, "shadow", summary.Side)
	assert.Equal(t, 4, summary.Selections)
	assert.Equal(t, 3, summary.WithOutcomes)
	assert.InDelta(t, 75.0, summary.CoveragePct, 1e-9)
	assert.Equal(t, 3, summary.Decided)
	assert.Equal(t, 2, summary.Wins)
	assert.Equal(t, 1, summary.Losses)
	assert.InDelta(t, 2.0/3.0, summary.WinRate, 1e-9)
	assert.InDelta(t, (2.0+1.0-1.5)/3.0, summary.MeanPnlPct, 1e-9)
}

// TestSummarizeOutcomes_UncoveredSelectionAffectsCoverageOnly: adding an
// uncovered selection changes coverage but neither the win rate nor the mean
// pnl -- nothing is imputed for it.
func TestSummarizeOutcomes_UncoveredSelectionAffectsCoverageOnly(t *testing.T) {
	covered := []Decision{
		decisionFor("A", 0.9, true),
		decisionFor("B", 0.8, true),
	}
	outcomes := map[uuid.UUID][]tradingstack.SimOutcome{
		covered[0].ScanResultID: {outcomeRow(covered[0].ScanResultID, floatPtr(3.0))},
		covered[1].ScanResultID: {outcomeRow(covered[1].ScanResultID, floatPtr(-1.0))},
	}

	baseline := SummarizeOutcomes("active", covered, outcomes)
	withUncovered := SummarizeOutcomes("active", append(covered, decisionFor("Z", 0.7, true)), outcomes)

	assert.InDelta(t, 100.0, baseline.CoveragePct, 1e-9)
	assert.InDelta(t, 2.0/3.0*100, withUncovered.CoveragePct, 1e-9)
	assert.Equal(t, 3, withUncovered.Selections)
	assert.Equal(t, 2, withUncovered.WithOutcomes)

	assert.Equal(t, baseline.Decided, withUncovered.Decided)
	assert.InDelta(t, baseline.WinRate, withUncovered.WinRate, 1e-9)
	assert.InDelta(t, baseline.MeanPnlPct, withUncovered.MeanPnlPct, 1e-9)
}

// TestSummarizeOutcomes_BreakevenExcludedFromDecided: pnl == 0 rows cover a
// selection but are excluded from the decided count and the win rate, per
// the ev-computation conventions. A nil pnl row also covers without deciding.
func TestSummarizeOutcomes_BreakevenExcludedFromDecided(t *testing.T) {
	selections := []Decision{
		decisionFor("A", 0.9, true),
		decisionFor("B", 0.8, true),
		decisionFor("C", 0.7, true),
	}
	outcomes := map[uuid.UUID][]tradingstack.SimOutcome{
		selections[0].ScanResultID: {outcomeRow(selections[0].ScanResultID, floatPtr(0.0))}, // breakeven
		selections[1].ScanResultID: {outcomeRow(selections[1].ScanResultID, floatPtr(1.0))}, // win
		selections[2].ScanResultID: {outcomeRow(selections[2].ScanResultID, nil)},           // no pnl recorded
	}

	summary := SummarizeOutcomes("shadow", selections, outcomes)

	assert.Equal(t, 3, summary.WithOutcomes, "breakeven and nil-pnl rows still cover their selections")
	assert.Equal(t, 1, summary.Decided)
	assert.Equal(t, 1, summary.Wins)
	assert.Equal(t, 0, summary.Losses)
	assert.InDelta(t, 1.0, summary.WinRate, 1e-9)
	assert.InDelta(t, 0.5, summary.MeanPnlPct, 1e-9, "mean over the two non-nil pnl rows")
}

// TestSummarizeOutcomes_EmptySelections: nothing selected -> all zeros, no
// division by zero.
func TestSummarizeOutcomes_EmptySelections(t *testing.T) {
	summary := SummarizeOutcomes("active", []Decision{decisionFor("A", 0.1, false)}, nil)

	assert.Equal(t, SideOutcomeSummary{Side: "active"}, summary)
}

// TestCompareOutcomes_CarriesBothSidesAndLimitation: the persisted comparison
// shape names each side and records the coverage limitation in the evidence
// itself.
func TestCompareOutcomes_CarriesBothSidesAndLimitation(t *testing.T) {
	active := []Decision{decisionFor("A", 0.9, true)}
	shadow := []Decision{decisionFor("B", 0.8, true)}
	outcomes := map[uuid.UUID][]tradingstack.SimOutcome{
		active[0].ScanResultID: {outcomeRow(active[0].ScanResultID, floatPtr(1.0))},
	}

	comparison := CompareOutcomes(active, shadow, outcomes)

	assert.Equal(t, "active", comparison.Active.Side)
	assert.Equal(t, "shadow", comparison.Shadow.Side)
	assert.Equal(t, 1, comparison.Active.WithOutcomes)
	assert.Equal(t, 0, comparison.Shadow.WithOutcomes)
	require.NotEmpty(t, comparison.Limitation)
	assert.Contains(t, comparison.Limitation, "never imputed")
}
