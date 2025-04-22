package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type Position struct {
	Quantity          float64 `json:"quantity"`
	CostBasis         float64 `json:"cost_basis"`
	PL                float64 `json:"pl"`
	MaintenanceMargin float64 `json:"maintenance_margin"`
	CurrentPrice      float64 `json:"current_price"`
}

func (p *Position) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("type assertion to []byte failed for CandleRepositoryRecord")
	}

	err := json.Unmarshal(bytes, &p)

	if err != nil {
		return fmt.Errorf("failed to unmarshal CandleRepositoryRecord: %w", err)
	}

	return nil
}

func (p Position) Value() (driver.Value, error) {
	return json.Marshal(p)
}
