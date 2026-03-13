package eventmodels

import "time"

type TradierPositionDTO struct {
	CostBasis    float64 `json:"cost_basis"`
	DateAcquired string  `json:"date_acquired"`
	ID           int     `json:"id"`
	Quantity     float64 `json:"quantity"`
	Symbol       string  `json:"symbol"`
}

func (dto TradierPositionDTO) ToModel() (*TradierPosition, error) {
	dateAcquired, err := time.Parse(time.RFC3339, dto.DateAcquired)
	if err != nil {
		return nil, err
	}

	return &TradierPosition{
		CostBasis:    dto.CostBasis,
		DateAcquired: dateAcquired,
		ID:           dto.ID,
		Quantity:     dto.Quantity,
		Symbol:       OptionSymbol(dto.Symbol),
	}, nil
}
