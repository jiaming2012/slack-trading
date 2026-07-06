package scanneropt

import (
	"fmt"
	"time"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting"
)

// SubmitToGate runs the overfitting gate over the evidence and persists the
// verdict -- pass or fail -- via the supplied verdict store. A failing verdict
// is not an error: it is audit evidence, persisted like any other, and the
// caller records the proposal as rejected_by_gate.
func SubmitToGate(
	evidence overfitting.Evidence,
	cfg overfitting.Config,
	computedAt time.Time,
	store overfitting.VerdictStore,
) (overfitting.Verdict, overfitting.OverfittingVerdict, error) {
	verdict, err := overfitting.RunGate(evidence, cfg)
	if err != nil {
		return overfitting.Verdict{}, overfitting.OverfittingVerdict{}, fmt.Errorf("scanneropt: gate run failed: %w", err)
	}

	row := overfitting.NewOverfittingVerdict(verdict, computedAt)
	if err := store.Persist(row); err != nil {
		return overfitting.Verdict{}, overfitting.OverfittingVerdict{}, fmt.Errorf("scanneropt: verdict persistence failed: %w", err)
	}

	return verdict, row, nil
}
