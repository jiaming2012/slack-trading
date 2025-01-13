package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type CandleRepositoryRecord []CandleRepositoryDTO

func (r *CandleRepositoryRecord) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("type assertion to []byte failed for CandleRepositoryRecord")
	}

	err := json.Unmarshal(bytes, &r)
	
	if err != nil {
		return fmt.Errorf("failed to unmarshal CandleRepositoryRecord: %w", err)
	}

	return nil
}

func (r CandleRepositoryRecord) Value() (driver.Value, error) {
	if len(r) == 0 {
		return nil, nil
	}
	return json.Marshal(r)
}
