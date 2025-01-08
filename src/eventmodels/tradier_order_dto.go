package eventmodels

import (
	"fmt"
	"time"
)

type TradierOrderDTO struct {
	ID                uint                 `json:"id"`
	Type              string               `json:"type"`
	Symbol            string               `json:"symbol"`
	Side              string               `json:"side"`
	Quantity          float64              `json:"quantity"`
	Status            string               `json:"status"`
	Duration          string               `json:"duration"`
	Price             float64              `json:"price"`
	AvgFillPrice      float64              `json:"avg_fill_price"`
	ExecQuantity      float64              `json:"exec_quantity"`
	LastFillPrice     float64              `json:"last_fill_price"`
	LastFillQuantity  float64              `json:"last_fill_quantity"`
	RemainingQuantity float64              `json:"remaining_quantity"`
	CreateDate        string               `json:"create_date"`
	TransactionDate   string               `json:"transaction_date"`
	Class             string               `json:"class"`
	Strategy          string               `json:"strategy"`
	OptionSymbol      *string              `json:"option_symbol"`
	Leg               []TradierOrderLegDTO `json:"leg"`
	ReasonDescription *string              `json:"reason_description"`
	Tag               string               `json:"tag"`
}

func (dto *TradierOrderDTO) ToTradierOrder() (*TradierOrder, error) {
	createDate, err := time.Parse(time.RFC3339, dto.CreateDate)
	if err != nil {
		return nil, fmt.Errorf("TradierOrderDTO:ToTradierOrder(): failed to parse create date: %w", err)
	}

	transactionDate, err := time.Parse(time.RFC3339, dto.TransactionDate)
	if err != nil {
		return nil, fmt.Errorf("TradierOrderDTO:toTradierOrder(): failed to parse transaction date: %w", err)
	}

	return &TradierOrder{
		ID:                dto.ID,
		Type:              dto.Type,
		Symbol:            dto.Symbol,
		Side:              dto.Side,
		Quantity:          dto.Quantity,
		Status:            dto.Status,
		Duration:          dto.Duration,
		Price:             dto.Price,
		AvgFillPrice:      dto.AvgFillPrice,
		ExecQuantity:      dto.ExecQuantity,
		LastFillPrice:     dto.LastFillPrice,
		LastFillQuantity:  dto.LastFillQuantity,
		RemainingQuantity: dto.RemainingQuantity,
		CreateDate:        createDate,
		TransactionDate:   transactionDate,
		Class:             dto.Class,
		OptionSymbol:      dto.OptionSymbol,
		Leg:               dto.Leg,
		Strategy:          dto.Strategy,
		ReasonDescription: dto.ReasonDescription,
		Tag:               dto.Tag,
	}, nil
}
