package fidelity

import (
	"context"

	log "github.com/sirupsen/logrus"
)

// Decision is the fidelity gate signal for a strategy.
type Decision string

const (
	// Proceed means the strategy is within tolerance and optimizers may consume
	// its simulator data.
	Proceed Decision = "proceed"
	// Pause means the strategy breached tolerance and optimizers must not consume
	// its simulator data.
	Pause Decision = "pause"
)

// Alerter raises an alert for a strategy that breached fidelity tolerance. It is
// injected so the alert path is deterministically observable in tests.
type Alerter interface {
	RaiseFidelityAlert(ctx context.Context, result Result)
}

// Gate derives a per-strategy gate decision from each Result's WithinTolerance
// verdict. Within-tolerance strategies yield Proceed and raise no alert;
// breaching strategies yield Pause and raise exactly one alert each through the
// injected Alerter. Gate only emits the decision and raises the alert — it never
// modifies or stops any optimizer.
func Gate(ctx context.Context, results []Result, a Alerter) map[string]Decision {
	decisions := make(map[string]Decision, len(results))
	for _, r := range results {
		if r.WithinTolerance {
			decisions[r.StrategyID] = Proceed
			continue
		}
		decisions[r.StrategyID] = Pause
		if a != nil {
			a.RaiseFidelityAlert(ctx, r)
		}
	}
	return decisions
}

// LogAlerter is the default Alerter. It raises breaches through logrus, which is
// already bridged to OpenTelemetry via otellogrus per the project setup.
type LogAlerter struct{}

// RaiseFidelityAlert logs a fidelity-breach warning carrying the strategy id and
// its drift figures.
func (LogAlerter) RaiseFidelityAlert(ctx context.Context, result Result) {
	log.WithContext(ctx).WithFields(log.Fields{
		"strategy_id":  result.StrategyID,
		"drift_score":  result.DriftScore,
		"drift_pnl":    result.DriftPnL,
		"drift_fill":   result.DriftFill,
		"period_start": result.Period.Start,
		"period_end":   result.Period.End,
	}).Warn("fidelity tolerance breached; pausing optimizer consumption for strategy")
}
