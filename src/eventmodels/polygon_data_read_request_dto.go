package eventmodels

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/polygon-io/client-go/rest/models"
)

type PolygonDataReadRequestDTO struct {
	BaseRequestEvent
	Symbol     string `json:"symbol"`
	From       string `json:"from"`
	To         string `json:"to"`
	Multiplier int    `json:"multiplier"`
	Timespan   string `json:"timespan"`
}

func (dto *PolygonDataReadRequestDTO) ToModel() (*PolygonDataReadRequest, error) {
	from, err := time.Parse(time.RFC3339, dto.From)
	if err != nil {
		return nil, fmt.Errorf("PolygonDataReadRequestDTO: ToModel: from: %w", err)
	}

	to, err := time.Parse(time.RFC3339, dto.To)
	if err != nil {
		return nil, fmt.Errorf("PolygonDataReadRequestDTO: ToModel: to: %w", err)
	}

	symbol := StockSymbol(dto.Symbol)

	return &PolygonDataReadRequest{
		Symbol:     symbol,
		From:       from,
		To:         to,
		Multiplier: dto.Multiplier,
		Timespan:   models.Timespan(dto.Timespan),
	}, nil
}

func (dto *PolygonDataReadRequestDTO) ParseHTTPRequest(r *http.Request) error {
	query := r.URL.Query()
	dto.Symbol = query.Get("symbol")
	dto.From = query.Get("from")
	dto.To = query.Get("to")
	dto.Timespan = query.Get("timespan")

	multiplierStr := query.Get("multiplier")
	multiplier, err := strconv.Atoi(multiplierStr)
	if err != nil {
		return fmt.Errorf("PolygonDataReadRequestDTO: ParseHTTPRequest: timeframe: %w", err)
	}

	dto.Multiplier = multiplier

	return nil
}

func (dto *PolygonDataReadRequestDTO) Validate(r *http.Request) error {
	if dto.Symbol == "" {
		return fmt.Errorf("PolygonDataReadRequestDTO: Validate: symbol is required")
	}

	if dto.From == "" {
		return fmt.Errorf("PolygonDataReadRequestDTO: Validate: from is required")
	}

	if dto.To == "" {
		return fmt.Errorf("PolygonDataReadRequestDTO: Validate: to is required")
	}

	if dto.Timespan == "" {
		return fmt.Errorf("PolygonDataReadRequestDTO: Validate: timespan is required")
	}

	if dto.Multiplier <= 0 {
		return fmt.Errorf("PolygonDataReadRequestDTO: Validate: multiplier must be greater than 0")
	}

	return nil
}
