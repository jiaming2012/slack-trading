package models

import (
	"fmt"
	"time"
)

// todo: refactor source into struct
type PlaygroundMeta struct {
	StartAt         time.Time             `json:"start_at"`
	EndAt           *time.Time            `json:"end_at"`
	Symbols         []string              `json:"symbols"`
	Tags            []string              `json:"tags"`
	InitialBalance  float64               `json:"starting_balance"`
	SourceBroker    string                `json:"source_broker"`
	SourceAccountId string                `json:"source_account_id"`
	LiveAccountType LiveAccountType       `json:"live_account_type"`
	Environment     PlaygroundEnvironment `json:"environment"`
}

func (p *PlaygroundMeta) HasTags(tags []string) bool {
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

func (p *PlaygroundMeta) Validate() error {
	if err := p.Environment.Validate(); err != nil {
		return fmt.Errorf("PlaygroundMeta.Validate: %w", err)
	}

	if p.Environment == PlaygroundEnvironmentLive {
		if p.StartAt.IsZero() {
			return fmt.Errorf("PlaygroundMeta.Validate: invalid start date: zero value")
		}

		if p.SourceBroker == "" {
			return fmt.Errorf("PlaygroundMeta.Validate: source broker is not set")
		}

		if p.SourceAccountId == "" {
			return fmt.Errorf("PlaygroundMeta.Validate: source account id is not set")
		}

		if err := p.LiveAccountType.Validate(); err != nil {
			return fmt.Errorf("PlaygroundMeta.Validate: failed to validate live account: %w", err)
		}
	} else if p.Environment == PlaygroundEnvironmentSimulator {
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

func (p *PlaygroundMeta) ToDTO() *PlaygroundMetaDTO {
	var liveAccountType *string
	if err := p.LiveAccountType.Validate(); err == nil {
		liveAccountType = new(string)
		*liveAccountType = string(p.LiveAccountType)
	}

	return &PlaygroundMetaDTO{
		StartDate:             p.StartAt.Format(time.RFC3339),
		EndDate:               p.EndAt.Format(time.RFC3339),
		Symbols:               p.Symbols,
		InitialBalance:        p.InitialBalance,
		Environment:           string(p.Environment),
		SourceBroker:          p.SourceBroker,
		SourceAccountId:       p.SourceAccountId,
		SourceLiveAccountType: liveAccountType,
	}
}
