package tradingstack

import "time"

// SimulatorFidelity mirrors the simulator_fidelity table (singular by design).
// It captures per-strategy drift between simulated and live performance.
type SimulatorFidelity struct {
	BaseModel
	ComputedAt      *time.Time `gorm:"column:computed_at;type:timestamptz"`
	StrategyID      *string    `gorm:"column:strategy_id;type:text"`
	PeriodStart     *time.Time `gorm:"column:period_start;type:timestamptz"`
	PeriodEnd       *time.Time `gorm:"column:period_end;type:timestamptz"`
	DriftPnl        *float64   `gorm:"column:drift_pnl;type:numeric"`
	DriftFill       *float64   `gorm:"column:drift_fill;type:numeric"`
	DriftScore      *float64   `gorm:"column:drift_score;type:numeric"`
	WithinTolerance bool       `gorm:"column:within_tolerance;type:boolean"`
}

// TableName pins the exact (singular) table name.
func (SimulatorFidelity) TableName() string {
	return "simulator_fidelity"
}
