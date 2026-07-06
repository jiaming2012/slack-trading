package shadowdeploy

import (
	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// SideOutcomeSummary is one side's coverage-explicit outcome comparison over
// its selected tickers. Outcomes are joined from already-persisted
// sim_outcomes rows via the observation's scan result id -- an outcome is
// NEVER fabricated, imputed, or simulated for an uncovered selection; such a
// selection reduces coverage and nothing else.
//
// Win/loss conventions mirror ev-computation: pnl_pct > 0 is a win, < 0 a
// loss, breakeven (== 0) is excluded from the decided count and win rate.
type SideOutcomeSummary struct {
	Side string `json:"side"` // "active" | "shadow"

	// Selections counts the side's selected decisions.
	Selections int `json:"selections"`

	// WithOutcomes counts selections having at least one sim_outcomes row;
	// CoveragePct = WithOutcomes / Selections x 100 (0 when nothing is
	// selected).
	WithOutcomes int     `json:"with_outcomes"`
	CoveragePct  float64 `json:"coverage_pct"`

	// Decided counts covered outcome rows with a non-nil, non-breakeven
	// pnl_pct; Wins and Losses partition it.
	Decided int `json:"decided"`
	Wins    int `json:"wins"`
	Losses  int `json:"losses"`

	// WinRate is Wins / Decided (0 when nothing is decided).
	WinRate float64 `json:"win_rate"`

	// MeanPnlPct is the mean pnl_pct over the covered selections' outcome
	// rows carrying a non-nil pnl_pct (0 when none do).
	MeanPnlPct float64 `json:"mean_pnl_pct"`
}

// OutcomeComparison pairs both sides' summaries -- the shape persisted into
// shadow_runs.outcome_summary_json.
type OutcomeComparison struct {
	Active SideOutcomeSummary `json:"active"`
	Shadow SideOutcomeSummary `json:"shadow"`

	// Limitation records, in the durable evidence itself, that coverage is
	// explicit rather than complete: selections without existing
	// sim_outcomes rows are counted in coverage only.
	Limitation string `json:"limitation"`
}

// outcomeCoverageLimitation is stamped into every persisted outcome summary.
const outcomeCoverageLimitation = "outcome comparison covers only selections that already have sim_outcomes rows; uncovered selections (shadow-only picks the simulator never processed) reduce coverage and are never imputed"

// SummarizeOutcomes computes one side's coverage-explicit outcome summary
// over its selected decisions, joining to existing sim_outcomes rows by scan
// result id. Pure: the caller loads the outcome rows.
func SummarizeOutcomes(side string, decisions []Decision, outcomesByScanResult map[uuid.UUID][]tradingstack.SimOutcome) SideOutcomeSummary {
	summary := SideOutcomeSummary{Side: side}

	var pnlSum float64
	var pnlCount int

	for _, d := range decisions {
		if !d.Selected {
			continue
		}
		summary.Selections++

		rows := outcomesByScanResult[d.ScanResultID]
		if len(rows) == 0 {
			// Uncovered selection: counted in coverage only.
			continue
		}
		summary.WithOutcomes++

		for _, row := range rows {
			if row.PnlPct == nil {
				continue
			}
			pnl := *row.PnlPct
			pnlSum += pnl
			pnlCount++
			switch {
			case pnl > 0:
				summary.Decided++
				summary.Wins++
			case pnl < 0:
				summary.Decided++
				summary.Losses++
			}
			// pnl == 0: breakeven, excluded from decided.
		}
	}

	if summary.Selections > 0 {
		summary.CoveragePct = float64(summary.WithOutcomes) / float64(summary.Selections) * 100
	}
	if summary.Decided > 0 {
		summary.WinRate = float64(summary.Wins) / float64(summary.Decided)
	}
	if pnlCount > 0 {
		summary.MeanPnlPct = pnlSum / float64(pnlCount)
	}

	return summary
}

// CompareOutcomes builds the persisted outcome comparison for both sides
// over the same outcome join.
func CompareOutcomes(active, shadow []Decision, outcomesByScanResult map[uuid.UUID][]tradingstack.SimOutcome) OutcomeComparison {
	return OutcomeComparison{
		Active:     SummarizeOutcomes("active", active, outcomesByScanResult),
		Shadow:     SummarizeOutcomes("shadow", shadow, outcomesByScanResult),
		Limitation: outcomeCoverageLimitation,
	}
}
