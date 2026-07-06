package overfitting

import (
	"time"

	"github.com/google/uuid"
)

// ProposalKind identifies which optimizer produced a proposal. Exactly two
// kinds exist; the gate rejects any other value.
type ProposalKind string

const (
	// ProposalKindScanner marks evidence submitted by the scanner-optimizer.
	ProposalKindScanner ProposalKind = "scanner"

	// ProposalKindStrategy marks evidence submitted by the strategy-optimizer.
	ProposalKindStrategy ProposalKind = "strategy"
)

// Valid reports whether the kind is one of the two known proposal kinds.
func (k ProposalKind) Valid() bool {
	return k == ProposalKindScanner || k == ProposalKindStrategy
}

// Performance summarizes the per-sample performance measure (e.g. ev-weighted
// pnl_pct) over one evaluation window. Mean and StdDev are the weighted mean
// and standard deviation of the per-sample measure; SampleSize is the number
// of samples in the window.
type Performance struct {
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	Mean        float64   `json:"mean"`
	StdDev      float64   `json:"std_dev"`
	SampleSize  int       `json:"sample_size"`
}

// FoldResult is one walk-forward fold: the model/parameters are fitted on
// [TrainStart, TrainEnd] and evaluated on [TestStart, TestEnd]. Params carries
// the per-fold fitted parameter values (feeding the cross-fold stability
// check); TestMetric is the fold's test-window performance measure.
type FoldResult struct {
	Index      int                `json:"index"`
	TrainStart time.Time          `json:"train_start"`
	TrainEnd   time.Time          `json:"train_end"`
	TestStart  time.Time          `json:"test_start"`
	TestEnd    time.Time          `json:"test_end"`
	Params     map[string]float64 `json:"params"`
	TestMetric float64            `json:"test_metric"`
}

// Evidence is the caller-supplied value an optimizer submits when asking the
// gate to judge a proposal. The gate operates only on this in-memory value and
// performs no database, network, or clock I/O; each consuming optimizer's own
// spec pins how it derives the evidence from the same rows it trained on.
type Evidence struct {
	// ProposalID identifies the proposal being judged.
	ProposalID uuid.UUID

	// ProposalKind is exactly one of ProposalKindScanner or
	// ProposalKindStrategy.
	ProposalKind ProposalKind

	// SampleSize is the count of decided labeled samples the proposal was
	// derived from.
	SampleSize int

	// TrialsCount is the count of candidate parameterizations evaluated
	// during the search that produced this proposal. A direct deterministic
	// derivation (no search) counts as 1.
	TrialsCount int

	// InSample and OutOfSample summarize the per-sample performance measure
	// over the training window and the held-out window respectively.
	InSample    Performance
	OutOfSample Performance

	// Folds are the chronologically ordered walk-forward folds.
	Folds []FoldResult

	// ProposedParams and BaselineParams are the comparable numeric knobs of
	// the proposed configuration and the current active baseline.
	ProposedParams map[string]float64
	BaselineParams map[string]float64
}
