package eventmodels

import (
	"fmt"
	"time"
)

type OptionCondorContractDTO struct {
	Timestamp time.Time `json:"timestamp"`
	Description string `json:"description"`
	Type OptionType `json:"type"`
	DebitPaid *float64 `json:"debit_paid"`
	CreditReceived *float64 `json:"credit_received"`
	LongCallTimestamp time.Time `json:"long_call_timestamp"`
	LongCallSymbol string `json:"long_call_symbol"`
	LongCallExpiration string `json:"long_call_expiration"`
	LongCallAvgFillPrice float64 `json:"long_call_avg_fill_price"`
	LongPutTimestamp time.Time `json:"long_put_timestamp"`
	LongPutSymbol string `json:"long_put_symbol"`
	LongPutExpiration string `json:"long_put_expiration"`
	LongPutAvgFillPrice float64 `json:"long_put_avg_fill_price"`
	ShortCallTimestamp time.Time `json:"short_call_timestamp"`
	ShortCallSymbol string `json:"short_call_symbol"`
	ShortCallExpiration string `json:"short_call_expiration"`
	ShortCallAvgFillPrice float64 `json:"short_call_avg_fill_price"`
	ShortPutTimestamp time.Time `json:"short_put_timestamp"`
	ShortPutSymbol string `json:"short_put_symbol"`
	ShortPutExpiration string `json:"short_put_expiration"`
	ShortPutAvgFillPrice float64 `json:"short_put_avg_fill_price"`
	Stats OptionStats `json:"stats"`
}

func (dto OptionCondorContractDTO) String() string {
	return fmt.Sprintf("LongCall: %v / ShortCall: %v / ShortPut: %v / LongPut: %v", dto.LongCallSymbol, dto.ShortCallSymbol, dto.ShortPutSymbol, dto.LongPutSymbol)
}

func (dto *OptionCondorContractDTO) GetExpiration() (time.Time, error) {
	longCallExpiration, err := time.Parse(time.RFC3339, dto.LongCallExpiration)
	if err != nil {
		return time.Time{}, fmt.Errorf("GetExpiration: failed to parse longExpiration %w", err)
	}

	shortCallExpiration, err := time.Parse(time.RFC3339, dto.ShortCallExpiration)
	if err != nil {
		return time.Time{}, fmt.Errorf("GetExpiration: failed to parse shortExpiration %w", err)
	}

	earliestExpiration := longCallExpiration
	if shortCallExpiration.Before(longCallExpiration) {
		earliestExpiration = shortCallExpiration
	}

	longPutExpiration, err := time.Parse(time.RFC3339, dto.LongPutExpiration)
	if err != nil {
		return time.Time{}, fmt.Errorf("GetExpiration: failed to parse longPutExpiration %w", err)
	}

	if longPutExpiration.Before(earliestExpiration) {
		earliestExpiration = longPutExpiration
	}

	shortPutExpiration, err := time.Parse(time.RFC3339, dto.ShortPutExpiration)
	if err != nil {
		return time.Time{}, fmt.Errorf("GetExpiration: failed to parse shortPutExpiration %w", err)
	}

	if shortPutExpiration.Before(earliestExpiration) {
		earliestExpiration = shortPutExpiration
	}

	return earliestExpiration, nil
}