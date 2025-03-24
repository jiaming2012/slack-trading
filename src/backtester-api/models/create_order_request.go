package models

import "fmt"

type CreateOrderRequest struct {
	Id              *uint               `json:"id"`
	ExternalOrderID *uint               `json:"external_order_id"`
	Symbol          string              `json:"symbol"`
	Class           OrderRecordClass    `json:"class"`
	Quantity        float64             `json:"quantity"`
	Side            TradierOrderSide    `json:"side"`
	OrderType       OrderRecordType     `json:"type"`
	Duration        OrderRecordDuration `json:"duration"`
	RequestedPrice  float64             `json:"requested_price"`
	Price           *float64            `json:"price"`
	StopPrice       *float64            `json:"stop_price"`
	Tag             string              `json:"tag"`
	CloseOrderId    *uint               `json:"close_order_id"`
	IsAdjustment    bool                `json:"is_adjustment"`
}

func (req *CreateOrderRequest) Validate() error {
	if err := req.Class.Validate(); err != nil {
		return fmt.Errorf("invalid class: %w", err)
	}

	if err := req.Side.Validate(); err != nil {
		return fmt.Errorf("invalid side: %w", err)
	}

	if req.Quantity <= 0 {
		return fmt.Errorf("quantity must be greater than 0")
	}

	if err := req.OrderType.Validate(); err != nil {
		return fmt.Errorf("invalid order type: %w", err)
	}

	if req.Price != nil && *req.Price <= 0 {
		return fmt.Errorf("price must be greater than 0")
	}

	if req.StopPrice != nil && *req.StopPrice <= 0 {
		return fmt.Errorf("stop price must be greater than 0")
	}

	if err := req.Duration.Validate(); err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	return nil
}
