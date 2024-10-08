package eventmodels

import (
	"fmt"
	"time"
)

type OptionSpreadContractDTO struct {
	Timestamp               time.Time    `json:"timestamp"`
	Description             string       `json:"description"`
	Type                    OptionType   `json:"type"`
	DebitPaid               *float64     `json:"debit_paid"`
	CreditReceived          *float64     `json:"credit_received"`
	LongOptionSymbol        OptionSymbol `json:"longOptionSymbol"`
	LongOptionTimestamp     time.Time    `json:"longOptionTimestamp"`
	LongOptionExpiration    string       `json:"longOptionExpiration"`
	LongOptionAvgFillPrice  float64      `json:"longOptionAvgFillPrice"`
	LongOptionStrikePrice   float64      `json:"longOptionStrikePrice"`
	ShortOptionTimestamp    time.Time    `json:"shortOptionTimestamp"`
	ShortOptionSymbol       OptionSymbol `json:"shortOptionSymbol"`
	ShortOptionExpiration   string       `json:"shortOptionExpiration"`
	ShortOptionAvgFillPrice float64      `json:"shortOptionAvgFillPrice"`
	ShortOptionStrikePrice  float64      `json:"shortOptionStrikePrice"`
	Stats                   OptionStats  `json:"stats"`
}

func (dto OptionSpreadContractDTO) String() string {
	return fmt.Sprintf("LongOptionSymbol: %v / ShortOptionSymbol: %v", dto.LongOptionSymbol, dto.ShortOptionSymbol)
}

func (dto *OptionSpreadContractDTO) GetExpiration() (time.Time, error) {
	longExpiration, err := time.Parse(time.RFC3339, dto.LongOptionExpiration)
	if err != nil {
		return time.Time{}, fmt.Errorf("GetExpiration: failed to parse longExpiration %w", err)
	}

	shortExpiration, err := time.Parse(time.RFC3339, dto.ShortOptionExpiration)
	if err != nil {
		return time.Time{}, fmt.Errorf("GetExpiration: failed to parse shortExpiration %w", err)
	}

	if longExpiration.Before(shortExpiration) {
		return shortExpiration, nil
	}

	return longExpiration, nil
}
