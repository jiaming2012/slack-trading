package tradingstack

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// validExitReasons is the closed set allowed for sim_outcomes.exit_reason. It is
// enforced both here (Go validation) and by a database CHECK constraint.
var validExitReasons = map[string]struct{}{
	"stop":        {},
	"target":      {},
	"timeout":     {},
	"signal_exit": {},
}

// SimOutcome mirrors the sim_outcomes table. scan_result_id references
// scan_results.id (foreign key added in MigrateTradingStack).
type SimOutcome struct {
	BaseModel
	ScanResultID uuid.UUID `gorm:"column:scan_result_id;type:uuid"`
	SimulatedAt  time.Time `gorm:"column:simulated_at;type:timestamptz;not null"`
	StrategyID   *string   `gorm:"column:strategy_id;type:text"`
	EntryPrice   *float64  `gorm:"column:entry_price;type:numeric"`
	ExitPrice    *float64  `gorm:"column:exit_price;type:numeric"`
	StopPrice    *float64  `gorm:"column:stop_price;type:numeric"`
	TargetPrice  *float64  `gorm:"column:target_price;type:numeric"`
	PnlPct       *float64  `gorm:"column:pnl_pct;type:numeric"`
	HoldDays     *int      `gorm:"column:hold_days;type:integer"`
	ExitReason   string    `gorm:"column:exit_reason;type:text"`
	MaxDrawdown  *float64  `gorm:"column:max_drawdown;type:numeric"`
	OutcomeLabel *string   `gorm:"column:outcome_label;type:text"`
}

// TableName pins the table name so GORM pluralization cannot alter it.
func (SimOutcome) TableName() string {
	return "sim_outcomes"
}

// BeforeSave validates exit_reason against the allowed set before the row
// reaches the database CHECK constraint.
func (o *SimOutcome) BeforeSave(tx *gorm.DB) error {
	if _, ok := validExitReasons[o.ExitReason]; !ok {
		return ErrInvalidExitReason
	}
	return nil
}
