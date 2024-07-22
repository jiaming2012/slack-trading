package eventmodels

import (
	"fmt"
	"strconv"
	"time"
)

type ExpectedProfitItemSpreadDTO struct {
	Description             string     `json:"description"`
	Type                    OptionType `json:"type"`
	LongOptionTimestamp     string     `json:"long_option_timestamp"`
	LongOptionSymbol        string     `json:"long_option_symbol"`
	LongOptionExpiration    string     `json:"long_option_expiration"`
	LongOptionAvgFillPrice  float64    `json:"long_option_avg_fill_price"`
	ShortOptionTimestamp    string     `json:"short_option_timestamp"`
	ShortOptionAvgFillPrice float64    `json:"short_option_avg_fill_price"`
	ShortOptionSymbol       string     `json:"short_option_symbol"`
	ShortOptionExpiration   string     `json:"short_option_expiration"`
	DebitPaid               string     `json:"debit_paid"`
	CreditReceived          string     `json:"credit_received"`
	ExpectedProfit          string     `json:"expected_profit"`
}

func (dto *ExpectedProfitItemSpreadDTO) ToModel() (*ExpectedProfitItemSpread, error) {
	if dto.DebitPaid == "NaN" {
		return nil, fmt.Errorf("ExpectedProfitItemSpreadDTO: ToModel: DebitPaid is NaN")
	}

	if dto.CreditReceived == "NaN" {
		return nil, fmt.Errorf("ExpectedProfitItemSpreadDTO: ToModel: CreditReceived is NaN")
	}

	if dto.ExpectedProfit == "NaN" {
		return nil, fmt.Errorf("ExpectedProfitItemSpreadDTO: ToModel: ExpectedProfit is NaN")
	}

	var debitPaid *float64
	var creditReceived *float64
	var expectedProfit float64

	if dto.DebitPaid != "" {
		debitPaidValue, err := strconv.ParseFloat(dto.DebitPaid, 64)
		if err != nil {
			return nil, fmt.Errorf("ExpectedProfitItemSpreadDTO: ToModel: failed to parse DebitPaid %w", err)
		}

		debitPaid = &debitPaidValue
	}

	if dto.CreditReceived != "" {
		creditReceivedValue, err := strconv.ParseFloat(dto.CreditReceived, 64)
		if err != nil {
			return nil, fmt.Errorf("ExpectedProfitItemSpreadDTO: ToModel: failed to parse CreditReceived %w", err)
		}

		creditReceived = &creditReceivedValue
	}

	if dto.ExpectedProfit != "" {
		expectedProfitValue, err := strconv.ParseFloat(dto.ExpectedProfit, 64)
		if err != nil {
			return nil, fmt.Errorf("ExpectedProfitItemSpreadDTO: ToModel: failed to parse ExpectedProfit %w", err)
		}

		expectedProfit = expectedProfitValue
	}

	longOptionTimestamp, err := time.Parse(time.RFC3339, dto.LongOptionTimestamp)
	if err != nil {
		return nil, fmt.Errorf("ExpectedProfitItemSpreadDTO: ToModel: failed to parse LongOptionTimestamp %w", err)
	}

	shortOptionTimestamp, err := time.Parse(time.RFC3339, dto.ShortOptionTimestamp)
	if err != nil {
		return nil, fmt.Errorf("ExpectedProfitItemSpreadDTO: ToModel: failed to parse ShortOptionTimestamp %w", err)
	}

	return &ExpectedProfitItemSpread{
		Description:             dto.Description,
		Type:                    dto.Type,
		LongOptionTimestamp:     longOptionTimestamp,
		LongOptionSymbol:        dto.LongOptionSymbol,
		LongOptionExpiration:    dto.LongOptionExpiration,
		LongOptionAvgFillPrice:  dto.LongOptionAvgFillPrice,
		ShortOptionTimestamp:    shortOptionTimestamp,
		ShortOptionSymbol:       dto.ShortOptionSymbol,
		ShortOptionExpiration:   dto.ShortOptionExpiration,
		ShortOptionAvgFillPrice: dto.ShortOptionAvgFillPrice,
		DebitPaid:               debitPaid,
		CreditReceived:          creditReceived,
		ExpectedProfit:          expectedProfit,
	}, nil
}
