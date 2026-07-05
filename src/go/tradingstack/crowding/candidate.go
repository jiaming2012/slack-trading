package crowding

import (
	"time"

	"github.com/google/uuid"
)

// ScanCandidate is the input shape for DetectCrowding: one scan_results row
// (identified by ScanResultID) plus the distinct sim_outcomes.strategy_id
// values that produced an outcome for it. A candidate with zero StrategyIDs
// had no simulating strategy and never counts as overlapping.
type ScanCandidate struct {
	ScanResultID uuid.UUID
	Ticker       string
	ScannedAt    time.Time
	StrategyIDs  []string
}
