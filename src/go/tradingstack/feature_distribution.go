package tradingstack

import "time"

// FeatureDistribution mirrors the feature_distributions table. The source SQL
// declares no primary key; a surrogate UUID id (from BaseModel) is added so
// GORM can manage CRUD. This is the single intentional deviation from the
// verbatim architecture SQL (documented in design.md).
type FeatureDistribution struct {
	BaseModel
	ComputedAt  *time.Time `gorm:"column:computed_at;type:timestamptz"`
	FeatureName *string    `gorm:"column:feature_name;type:text"`
	Mean        *float64   `gorm:"column:mean;type:numeric"`
	StdDev      *float64   `gorm:"column:std_dev;type:numeric"`
	P25         *float64   `gorm:"column:p25;type:numeric"`
	P75         *float64   `gorm:"column:p75;type:numeric"`
}

// TableName pins the table name so GORM pluralization cannot alter it.
func (FeatureDistribution) TableName() string {
	return "feature_distributions"
}
