package tradingstack

import "time"

// StrategyEvWeight mirrors the strategy_ev_weights table. ev_slope may be
// negative (decaying expected value); all EV fields round-trip as numerics.
type StrategyEvWeight struct {
	BaseModel
	ComputedAt *time.Time `gorm:"column:computed_at;type:timestamptz"`
	StrategyID *string    `gorm:"column:strategy_id;type:text"`
	Regime     *string    `gorm:"column:regime;type:text"`
	Ev30d      *float64   `gorm:"column:ev_30d;type:numeric"`
	Ev90d      *float64   `gorm:"column:ev_90d;type:numeric"`
	EvSlope    *float64   `gorm:"column:ev_slope;type:numeric"`
	EvWeight   *float64   `gorm:"column:ev_weight;type:numeric"`
}

// TableName pins the table name so GORM pluralization cannot alter it.
func (StrategyEvWeight) TableName() string {
	return "strategy_ev_weights"
}
