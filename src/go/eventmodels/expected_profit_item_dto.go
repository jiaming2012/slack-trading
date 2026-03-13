package eventmodels

import (
	"fmt"
	"strconv"
)

type ExpectedProfitItemDTO struct {
	Description    string `json:"description"`
	DebitPaid      string `json:"debit_paid"`
	CreditReceived string `json:"credit_received"`
	ExpectedProfit string `json:"expected_profit"`
}

func (dto *ExpectedProfitItemDTO) ToModel() (*ExpectedProfitItem, error) {
	if dto.DebitPaid == "NaN" {
		return nil, fmt.Errorf("ExpectedProfitItemDTO: ToModel: DebitPaid is NaN")
	}

	if dto.CreditReceived == "NaN" {
		return nil, fmt.Errorf("ExpectedProfitItemDTO: ToModel: CreditReceived is NaN")
	}

	if dto.ExpectedProfit == "NaN" {
		return nil, fmt.Errorf("ExpectedProfitItemDTO: ToModel: ExpectedProfit is NaN")
	}

	var debitPaid *float64
	var creditReceived *float64
	var expectedProfit float64

	if dto.DebitPaid != "" {
		debitPaidValue, err := strconv.ParseFloat(dto.DebitPaid, 64)
		if err != nil {
			return nil, fmt.Errorf("ExpectedProfitItemDTO: ToModel: failed to parse DebitPaid %w", err)
		}

		debitPaid = &debitPaidValue
	}

	if dto.CreditReceived != "" {
		creditReceivedValue, err := strconv.ParseFloat(dto.CreditReceived, 64)
		if err != nil {
			return nil, fmt.Errorf("ExpectedProfitItemDTO: ToModel: failed to parse CreditReceived %w", err)
		}

		creditReceived = &creditReceivedValue
	}

	if dto.ExpectedProfit != "" {
		expectedProfitValue, err := strconv.ParseFloat(dto.ExpectedProfit, 64)
		if err != nil {
			return nil, fmt.Errorf("ExpectedProfitItemDTO: ToModel: failed to parse ExpectedProfit %w", err)
		}

		expectedProfit = expectedProfitValue
	}

	return &ExpectedProfitItem{
		Description:    dto.Description,
		DebitPaid:      debitPaid,
		CreditReceived: creditReceived,
		ExpectedProfit: expectedProfit,
	}, nil
}
