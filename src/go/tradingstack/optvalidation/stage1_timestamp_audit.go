package optvalidation

import (
	"time"

	"github.com/google/uuid"
)

// Violation identifies a TrainingRow excluded by the timestamp-audit stage
// because its DataAsOf is strictly after its ScannedAt -- a lookahead-bias
// violation. It carries enough identifying information to audit the
// exclusion later.
type Violation struct {
	ScanResultID uuid.UUID
	StrategyID   string
	Ticker       string
	ScannedAt    time.Time
	DataAsOf     time.Time
}

// TimestampAudit excludes every TrainingRow whose DataAsOf is strictly after
// its ScannedAt, returning the surviving rows and the excluded violations
// separately. Pure function: no I/O, no mutation of the input slice, and
// deterministic ordering of both outputs (input order preserved).
func TimestampAudit(rows []TrainingRow) (kept []TrainingRow, violations []Violation) {
	kept = make([]TrainingRow, 0, len(rows))
	violations = make([]Violation, 0)

	for _, r := range rows {
		if r.DataAsOf.After(r.ScannedAt) {
			violations = append(violations, Violation{
				ScanResultID: r.ScanResultID,
				StrategyID:   r.StrategyID,
				Ticker:       r.Ticker,
				ScannedAt:    r.ScannedAt,
				DataAsOf:     r.DataAsOf,
			})
			continue
		}
		kept = append(kept, r)
	}

	return kept, violations
}
