package models

import (
	"fmt"

	"github.com/google/uuid"
)

type CreateOrderRequest struct {
	Id              *uint               `json:"id"`
	ClientRequestID *string             `json:"client_request_id"`
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
	IsSystemOrder   bool                `json:"is_system_order"`
	Attributes      map[string]string   `json:"attributes"`
	PreviousBalance *float64            `json:"previous_balance"`
	SignalID        *uuid.UUID          `json:"signal_id"`
}

func (req *CreateOrderRequest) Validate() error {
	if err := req.Class.Validate(); err != nil {
		return fmt.Errorf("invalid class: %w", err)
	}

	if req.Class == OrderRecordClassOption && req.Symbol[:2] != "O:" {
		return fmt.Errorf("invalid option symbol format: %s. Must start with 'O:'", req.Symbol)
	}

	if err := req.Side.Validate(req.Class); err != nil {
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

	// Reserved-tag boundary (wire-companion-stops): the companion-stop prefix
	// is the recursion guard and the durable idempotency association for
	// broker-held protective stops. Enforced here so EVERY order-placement
	// ingress (single-leg RPC, multi-leg RPC, REST) inherits the rejection —
	// companion stops themselves are placed at the Broker seam via
	// PlaceOrderRequest and never pass through this validation.
	if IsCompanionStopOrderTag(req.Tag) {
		return fmt.Errorf("tag %q uses the reserved companion-stop prefix; choose a different tag", req.Tag)
	}

	return nil
}
