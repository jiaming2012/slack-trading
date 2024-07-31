package eventmodels

import (
	"fmt"
	"strconv"
	"time"
)

type ExpectedProfitItemCondorDTO struct {
	Description           string     `json:"description"`
	Type                  OptionType `json:"type"`
	LongCallTimestamp     string     `json:"long_call_timestamp"`
	LongCallSymbol        string     `json:"long_call_symbol"`
	LongCallExpiration    string     `json:"long_call_expiration"`
	LongCallAvgFillPrice  float64    `json:"long_call_avg_fill_price"`
	LongPutTimestamp      string     `json:"long_put_timestamp"`
	LongPutSymbol         string     `json:"long_put_symbol"`
	LongPutExpiration     string     `json:"long_put_expiration"`
	LongPutAvgFillPrice   float64    `json:"long_put_avg_fill_price"`
	ShortCallTimestamp    string     `json:"short_call_timestamp"`
	ShortCallSymbol       string     `json:"short_call_symbol"`
	ShortCallExpiration   string     `json:"short_call_expiration"`
	ShortCallAvgFillPrice float64    `json:"short_call_avg_fill_price"`
	ShortPutTimestamp     string     `json:"short_put_timestamp"`
	ShortPutSymbol        string     `json:"short_put_symbol"`
	ShortPutExpiration    string     `json:"short_put_expiration"`
	ShortPutAvgFillPrice  float64    `json:"short_put_avg_fill_price"`
	CreditReceived        string     `json:"credit_received"`
	DebitPaid             string     `json:"debit_paid"`
	ExpectedProfit        string     `json:"expected_profit"`
}

func (dto *ExpectedProfitItemCondorDTO) ToModel() (*ExpectedProfitItemCondor, error) {
	if dto.DebitPaid == "NaN" {
		return nil, fmt.Errorf("ExpectedProfitItemCondorDTO: ToModel: DebitPaid is NaN")
	}

	if dto.CreditReceived == "NaN" {
		return nil, fmt.Errorf("ExpectedProfitItemCondorDTO: ToModel: CreditReceived is NaN")
	}

	if dto.ExpectedProfit == "NaN" {
		return nil, fmt.Errorf("ExpectedProfitItemCondorDTO: ToModel: ExpectedProfit is NaN")
	}

	var debitPaid *float64
	var creditReceived *float64
	var expectedProfit float64

	if dto.DebitPaid != "" {
		debitPaidValue, err := strconv.ParseFloat(dto.DebitPaid, 64)
		if err != nil {
			return nil, fmt.Errorf("ExpectedProfitItemCondorDTO: ToModel: failed to parse DebitPaid %w", err)
		}

		debitPaid = &debitPaidValue
	}

	if dto.CreditReceived != "" {
		creditReceivedValue, err := strconv.ParseFloat(dto.CreditReceived, 64)
		if err != nil {
			return nil, fmt.Errorf("ExpectedProfitItemCondorDTO: ToModel: failed to parse CreditReceived %w", err)
		}

		creditReceived = &creditReceivedValue
	}

	if dto.ExpectedProfit != "" {
		expectedProfitValue, err := strconv.ParseFloat(dto.ExpectedProfit, 64)
		if err != nil {
			return nil, fmt.Errorf("ExpectedProfitItemCondorDTO: ToModel: failed to parse ExpectedProfit %w", err)
		}

		expectedProfit = expectedProfitValue
	}

	longCallTimestamp, err := time.Parse(time.RFC3339, dto.LongCallTimestamp)
	if err != nil {
		return nil, fmt.Errorf("ExpectedProfitItemCondorDTO: ToModel: failed to parse LongCallTimestamp %w", err)
	}

	longPutTimestamp, err := time.Parse(time.RFC3339, dto.LongPutTimestamp)
	if err != nil {
		return nil, fmt.Errorf("ExpectedProfitItemCondorDTO: ToModel: failed to parse LongPutTimestamp %w", err)
	}

	shortCallTimestamp, err := time.Parse(time.RFC3339, dto.ShortCallTimestamp)
	if err != nil {
		return nil, fmt.Errorf("ExpectedProfitItemCondorDTO: ToModel: failed to parse ShortCallTimestamp %w", err)
	}

	shortPutTimestamp, err := time.Parse(time.RFC3339, dto.ShortPutTimestamp)
	if err != nil {
		return nil, fmt.Errorf("ExpectedProfitItemCondorDTO: ToModel: failed to parse ShortPutTimestamp %w", err)
	}

	return &ExpectedProfitItemCondor{
		Description:         dto.Description,
		Type:                dto.Type,
		LongCallTimestamp:   longCallTimestamp,
		LongCallSymbol:      dto.LongCallSymbol,
		LongCallExpiration:  dto.LongCallExpiration,
		LongCallAvgFillPrice: dto.LongCallAvgFillPrice,
		LongPutTimestamp:    longPutTimestamp,
		LongPutSymbol:       dto.LongPutSymbol,
		LongPutExpiration:   dto.LongPutExpiration,
		LongPutAvgFillPrice: dto.LongPutAvgFillPrice,
		ShortCallTimestamp:  shortCallTimestamp,
		ShortCallSymbol:     dto.ShortCallSymbol,
		ShortCallExpiration: dto.ShortCallExpiration,
		ShortCallAvgFillPrice: dto.ShortCallAvgFillPrice,
		ShortPutTimestamp:    shortPutTimestamp,
		ShortPutSymbol:      dto.ShortPutSymbol,
		ShortPutExpiration:  dto.ShortPutExpiration,
		ShortPutAvgFillPrice: dto.ShortPutAvgFillPrice,
		CreditReceived:      creditReceived,
		DebitPaid:           debitPaid,
		ExpectedProfit:      expectedProfit,
	}, nil
}
