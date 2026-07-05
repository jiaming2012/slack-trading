package safety

// Source identifies who engaged the halt: a human operator (manual) or an
// anomaly guard (auto). It is recorded on the persisted state and surfaced in
// the status response so an operator can tell an auto-halt (which requires a
// cooldown acknowledgment) apart from a manual halt.
type Source string

const (
	// SourceManual marks a halt engaged by an operator via the REST/Taskfile
	// surface. A manual halt carries no cooldown-acknowledgment requirement and
	// may be released directly.
	SourceManual Source = "manual"

	// SourceAuto marks a halt engaged automatically by an anomaly guard. An
	// auto-halt sets the cooldown-acknowledgment requirement and cannot be
	// released until an operator acknowledges it.
	SourceAuto Source = "auto"
)

// HaltState is the complete, persistable state of the halt controller. It is
// the single source of truth for "may orders be submitted right now?" and is
// written through a HaltStateStore on every transition so a restart resumes the
// exact prior state.
type HaltState struct {
	// Engaged is true when order submission is halted.
	Engaged bool `json:"engaged"`

	// Reason is a human-readable description of why the halt is engaged
	// (operator note or the name of the guard that tripped). Empty when clear.
	Reason string `json:"reason"`

	// Source records whether the halt was engaged manually or automatically.
	// Empty when clear.
	Source Source `json:"source"`

	// AckRequired is true when an auto-halt is awaiting an explicit operator
	// acknowledgment before it may be released (the cooldown requirement). It is
	// always false for a manual halt and while clear.
	AckRequired bool `json:"ack_required"`
}

// HaltStateStore persists and restores the halt controller's state. The
// default implementation is file-backed JSON (halt_store_file.go); an in-memory
// implementation (halt_store_memory.go) is provided for tests. No
// implementation requires a live production database connection, satisfying the
// constraint that the kill switch works without the prod DB.
type HaltStateStore interface {
	// Load returns the persisted state. A store with nothing persisted yet
	// SHALL return a zero-value (clear) HaltState and a nil error, so a fresh
	// controller defaults to clear.
	Load() (HaltState, error)

	// Save persists the given state, overwriting any prior state.
	Save(state HaltState) error
}
