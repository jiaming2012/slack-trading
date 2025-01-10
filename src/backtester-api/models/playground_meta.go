package models

import (
	"fmt"
	"time"
)

type PlaygroundMeta struct {
	StartAt         time.Time             `json:"start_at"`
	EndAt           *time.Time            `json:"end_at"`
	Symbols         []string              `json:"symbols"`
	StartingBalance float64               `json:"starting_balance"`
	Environment     PlaygroundEnvironment `json:"environment"`
}

func (p *PlaygroundMeta) Validate() error {
	if err := p.Environment.Validate(); err != nil {
		return fmt.Errorf("PlaygroundMeta.Validate: %w", err)
	}

	if p.Environment == PlaygroundEnvironmentLive {
		if p.StartAt.IsZero() {
			return fmt.Errorf("PlaygroundMeta.Validate: invalid start date: zero value")
		}
	} else {
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

	if p.StartingBalance <= 0 {
		return fmt.Errorf("PlaygroundMeta.Validate: invalid starting balance")
	}

	return nil
}

func (p *PlaygroundMeta) ToDTO() *PlaygroundMetaDTO {
	return &PlaygroundMetaDTO{
		StartDate:       p.StartAt.Format(time.RFC3339),
		EndDate:         p.EndAt.Format(time.RFC3339),
		Symbols:         p.Symbols,
		StartingBalance: p.StartingBalance,
		Environment:     string(p.Environment),
	}
}
