package models

import (
	"fmt"
	"time"

	"github.com/lib/pq"
)

// todo: refactor source into struct
type Meta struct {
	PlaygroundId          string         `json:"playground_id" gorm:"-"`
	ReconcilePlaygroundId *string        `json:"reconcile_playground_id" gorm:"-"`
	StartAt               time.Time      `json:"start_at" gorm:"column:start_at;type:timestamptz;not null"`
	ClientID              *string        `json:"client_id" gorm:"column:client_id;type:text;unique"`
	EndAt                 *time.Time     `json:"end_at" gorm:"column:end_at;type:timestamptz"`
	Symbols               pq.StringArray `json:"symbols" gorm:"column:symbols;type:text[]"`
	Tags                  pq.StringArray `json:"tags" gorm:"column:tags;type:text[]"`
	InitialBalance        float64        `json:"starting_balance" gorm:"column:starting_balance;type:numeric;not null"`
	SourceBroker          string         `json:"source_broker" gorm:"column:source_broker;type:text;not null"`
	SourceAccountId       string         `json:"source_account_id" gorm:"column:source_account_id;type:text;not null"`

	// Mode is the operator-selected preset (Simulation, Paper, Margin). It is
	// in-memory only: on load it is hydrated from the legacy columns via
	// HydrateMode, and it is empty for internal reconciliation containers
	// (IsReconciliation), which are not a Mode.
	Mode Mode `json:"mode" gorm:"-"`

	// Role is the internal per-account tag behind the Broker seam. It persists
	// the legacy live_account_type column byte-identically (a legacy "mock"
	// row stays "mock" on disk).
	Role AccountRole `json:"live_account_type" gorm:"column:live_account_type;type:text;not null"`

	// LegacyEnv persists the legacy environment column
	// (simulator|live|reconcile) byte-identically. It is a persistence/RPC
	// boundary artifact kept in sync with Mode by NewMeta /
	// NewReconciliationMeta / HydrateMode; business logic must branch on
	// Mode / Role / IsReconciliation, never on this field.
	LegacyEnv string `json:"environment" gorm:"column:environment;type:text;not null"`

	CurrentTime time.Time `json:"current_time" gorm:"-"`
}

// NewMeta builds metadata for an operator-facing playground in the given
// mode. The internal Role is filled in by the construction path
// (PopulatePlayground) once the account source is known.
func NewMeta(mode Mode, tags []string) *Meta {
	return &Meta{
		Mode:      mode,
		LegacyEnv: mode.LegacyEnvironment(),
		Tags:      tags,
	}
}

// NewReconciliationMeta builds metadata for an internal reconciliation
// container — the netting layer behind the Broker seam. It carries no Mode;
// the operator can neither create nor select it.
func NewReconciliationMeta(tags []string) *Meta {
	return &Meta{
		LegacyEnv: LegacyEnvReconcile,
		Tags:      tags,
	}
}

// IsReconciliation reports whether this playground is an internal
// reconciliation container (legacy environment="reconcile"). Such rows are
// never a Mode.
func (p *Meta) IsReconciliation() bool {
	return p.LegacyEnv == LegacyEnvReconcile
}

// HydrateMode recomputes Mode from the persisted legacy columns. It is
// invoked by the Playground AfterFind GORM hook so every row loaded from
// Postgres carries a coherent in-memory Mode; unrecognized legacy
// combinations surface an explicit error rather than a silent default.
func (p *Meta) HydrateMode() error {
	mode, internalReconcile, err := ModeFromLegacy(p.LegacyEnv, string(p.Role))
	if err != nil {
		return fmt.Errorf("Meta.HydrateMode: %w", err)
	}

	if internalReconcile {
		p.Mode = ""
		return nil
	}

	p.Mode = mode
	return nil
}

func (p *Meta) HasTags(tags []string) bool {
	for _, tag := range tags {
		found := false
		for _, t := range p.Tags {
			if tag == t {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

func (p *Meta) Validate() error {
	if !p.IsReconciliation() {
		if err := p.Mode.Validate(); err != nil {
			return fmt.Errorf("PlaygroundMeta.Validate: %w", err)
		}
	}

	if err := p.Role.Validate(); err != nil {
		return fmt.Errorf("savePlaygroundSession: invalid account role: %w", err)
	}

	if p.PlaygroundId == "" {
		return fmt.Errorf("PlaygroundMeta.Validate: playground id is not set")
	}

	if p.Mode.IsRealtime() {
		if p.ReconcilePlaygroundId == nil {
			return fmt.Errorf("PlaygroundMeta.Validate: reconcile playground id is not set")
		}

		if p.StartAt.IsZero() {
			return fmt.Errorf("PlaygroundMeta.Validate: invalid start date: zero value")
		}

		if p.SourceBroker == "" {
			return fmt.Errorf("PlaygroundMeta.Validate: source broker is not set")
		}

		if p.SourceAccountId == "" {
			return fmt.Errorf("PlaygroundMeta.Validate: source account id is not set")
		}

		if err := p.Role.Validate(); err != nil {
			return fmt.Errorf("PlaygroundMeta.Validate: failed to validate account role: %w", err)
		}
	} else if p.Mode == ModeSimulation {
		if p.StartAt.IsZero() {
			return fmt.Errorf("PlaygroundMeta.Validate: invalid start date: zero value")
		}

		if p.EndAt == nil {
			return fmt.Errorf("PlaygroundMeta.Validate: end date is not set")
		}

		if p.EndAt.IsZero() {
			return fmt.Errorf("PlaygroundMeta.Validate: invalid end date: zero value")
		}

		if p.StartAt.After(*p.EndAt) {
			return fmt.Errorf("PlaygroundMeta.Validate: start date is after end date")
		}
	}

	if p.InitialBalance < 0 {
		return fmt.Errorf("PlaygroundMeta.Validate: invalid starting balance")
	}

	return nil
}
