package eventmodels

import (
	"fmt"
	"strconv"
)

type ExpectedProfitItemSpreadDTO struct {
	Description           string     `json:"description"`
	Type                  OptionType `json:"type"`
	LongOptionSymbol      string     `json:"long_option_symbol"`
	LongOptionExpiration  string     `json:"long_option_expiration"`
	ShortOptionSymbol     string     `json:"short_option_symbol"`
	ShortOptionExpiration string     `json:"short_option_expiration"`
	DebitPaid             string     `json:"debit_paid"`
	CreditReceived        string     `json:"credit_received"`
	ExpectedProfit        string     `json:"expected_profit"`
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

	return &ExpectedProfitItemSpread{
		Description:           dto.Description,
		Type:                  dto.Type,
		LongOptionSymbol:      dto.LongOptionSymbol,
		LongOptionExpiration:  dto.LongOptionExpiration,
		ShortOptionSymbol:     dto.ShortOptionSymbol,
		ShortOptionExpiration: dto.ShortOptionExpiration,
		DebitPaid:             debitPaid,
		CreditReceived:        creditReceived,
		ExpectedProfit:        expectedProfit,
	}, nil
}
