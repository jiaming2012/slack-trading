package optvalidation

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// driftPeriod is an inclusive [start, end] window during which a strategy's
// simulator was found to be out of tolerance with live performance.
type driftPeriod struct {
	start time.Time
	end   time.Time
}

// FidelityGate indexes the within_tolerance=false periods in fidelity by
// strategy id, then drops every TrainingRow whose strategy matches a
// high-drift period and whose DataAsOf falls within that period's bounds
// (inclusive). Rows for strategies with no SimulatorFidelity record, or
// whose DataAsOf falls outside every high-drift period, survive. An empty
// fidelity input drops nothing.
func FidelityGate(rows []TrainingRow, fidelity []tradingstack.SimulatorFidelity) (kept, dropped []TrainingRow) {
	periodsByStrategy := make(map[string][]driftPeriod)
	for _, f := range fidelity {
		if f.WithinTolerance || f.StrategyID == nil || f.PeriodStart == nil || f.PeriodEnd == nil {
			continue
		}
		periodsByStrategy[*f.StrategyID] = append(periodsByStrategy[*f.StrategyID], driftPeriod{
			start: *f.PeriodStart,
			end:   *f.PeriodEnd,
		})
	}

	kept = make([]TrainingRow, 0, len(rows))
	dropped = make([]TrainingRow, 0)

	for _, r := range rows {
		if inHighDriftPeriod(r, periodsByStrategy[r.StrategyID]) {
			dropped = append(dropped, r)
			continue
		}
		kept = append(kept, r)
	}

	return kept, dropped
}

func inHighDriftPeriod(r TrainingRow, periods []driftPeriod) bool {
	for _, p := range periods {
		if !r.DataAsOf.Before(p.start) && !r.DataAsOf.After(p.end) {
			return true
		}
	}
	return false
}
