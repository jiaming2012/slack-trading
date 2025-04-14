package models

import (
	"fmt"
	"time"

	"github.com/lib/pq"
)

// todo: refactor source into struct
type Meta struct {
	PlaygroundId          string                `json:"playground_id" gorm:"column:playground_id;type:text;primaryKey"`
	ReconcilePlaygroundId *string               `json:"reconcile_playground_id" gorm:"column:reconcile_playground_id;type:text"`
	StartAt               time.Time             `json:"start_at" gorm:"column:start_at;type:timestamptz;not null"`
	ClientID              *string               `json:"client_id" gorm:"column:client_id;type:text;unique"`
	EndAt                 *time.Time            `json:"end_at" gorm:"column:end_at;type:timestamptz"`
	Symbols               pq.StringArray        `json:"symbols" gorm:"column:symbols;type:text[]"`
	Tags                  pq.StringArray        `json:"tags" gorm:"column:tags;type:text[]"`
	InitialBalance        float64               `json:"starting_balance" gorm:"column:starting_balance;type:numeric;not null"`
	SourceBroker          string                `json:"source_broker" gorm:"column:source_broker;type:text;not null"`
	SourceAccountId       string                `json:"source_account_id" gorm:"column:source_account_id;type:text;not null"`
	LiveAccountType       LiveAccountType       `json:"live_account_type" gorm:"column:live_account_type;type:text;not null"`
	Environment           PlaygroundEnvironment `json:"environment" gorm:"column:environment;type:text;not null"`
}

func NewMeta(env PlaygroundEnvironment, tags []string) *Meta {
	return &Meta{
		Environment: env,
		Tags:        tags,
	}
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
	if err := p.Environment.Validate(); err != nil {
		return fmt.Errorf("PlaygroundMeta.Validate: %w", err)
	}

	if err := p.LiveAccountType.Validate(); err != nil {
		return fmt.Errorf("savePlaygroundSession: invalid live account type: %w", err)
	}

	if p.PlaygroundId == "" {
		return fmt.Errorf("PlaygroundMeta.Validate: playground id is not set")
	}

	if p.Environment == PlaygroundEnvironmentLive {
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

// func (p *Meta) ToDTO() *PlaygroundMetaDTO {
// 	var liveAccountType *string
// 	if err := p.LiveAccountType.Validate(); err == nil {
// 		liveAccountType = new(string)
// 		*liveAccountType = string(p.LiveAccountType)
// 	}

// 	return &PlaygroundMetaDTO{
// 		StartDate:             p.StartAt.Format(time.RFC3339),
// 		EndDate:               p.EndAt.Format(time.RFC3339),
// 		Symbols:               p.Symbols,
// 		InitialBalance:        p.InitialBalance,
// 		Environment:           string(p.Environment),
// 		SourceBroker:          p.SourceBroker,
// 		SourceAccountId:       p.SourceAccountId,
// 		SourceLiveAccountType: liveAccountType,
// 	}
// }
