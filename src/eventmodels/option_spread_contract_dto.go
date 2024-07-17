package eventmodels

import (
	"fmt"
	"time"
)

type OptionSpreadContractDTO struct {
	Description           string       `json:"description"`
	Type                  OptionType   `json:"type"`
	DebitPaid             *float64     `json:"debit_paid"`
	CreditReceived        *float64     `json:"credit_received"`
	LongOptionSymbol      OptionSymbol `json:"longOptionSymbol"`
	LongOptionExpiration  string       `json:"longOptionExpiration"`
	ShortOptionSymbol     OptionSymbol `json:"shortOptionSymbol"`
	ShortOptionExpiration string       `json:"shortOptionExpiration"`
	Stats                 OptionStats  `json:"stats"`
}

func (dto *OptionSpreadContractDTO) GetExpiration() (time.Time, error) {
	longExpiration, err := time.Parse("2006-01-02T15:04:05Z", dto.LongOptionExpiration)
	if err != nil {
		return time.Time{}, fmt.Errorf("GetExpiration: failed to parse longExpiration %w", err)
	}
	
	shortExpiration, err := time.Parse("2006-01-02T15:04:05Z", dto.ShortOptionExpiration)
	if err != nil {
		return time.Time{}, fmt.Errorf("GetExpiration: failed to parse shortExpiration %w", err)
	}

	if longExpiration.Before(shortExpiration) {
		return shortExpiration, nil
	}

	return longExpiration, nil
}
