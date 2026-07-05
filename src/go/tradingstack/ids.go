package tradingstack

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseModel provides the shared UUID primary key and a BeforeCreate hook that
// assigns a fresh UUID when the id is unset. Embedding this into every
// trading-stack model keeps id generation self-contained (no reliance on the
// uuid-ossp Postgres extension) and consistent across the package.
type BaseModel struct {
	ID uuid.UUID `gorm:"column:id;type:uuid;primaryKey"`
}

// BeforeCreate populates the primary key with a new UUID when it is the zero
// value, so callers may leave ID unset and still get a stable, generated id.
func (b *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}
