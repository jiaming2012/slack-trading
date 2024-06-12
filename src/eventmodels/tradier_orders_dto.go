package eventmodels

import (
	"encoding/json"
	"errors"
	"fmt"
)

type TradierOrdersDTO struct {
	Orders json.RawMessage `json:"orders"`
}

func (dto *TradierOrdersDTO) Parse() ([]*TradierOrderDTO, error) {
	var err error

	// check if orders is null
	var msg string

	if err = json.Unmarshal(dto.Orders, &msg); err == nil {
		if msg == "null" {
			return []*TradierOrderDTO{}, nil
		} else {
			return nil, fmt.Errorf("TradierOrdersDTO:Parse(): failed to unmarshal orders: unknown message: %s", msg)
		}
	} else {
		var unmarshalTypeError *json.UnmarshalTypeError
		if errors.As(err, &unmarshalTypeError) {
			// log.Debug("TradierOrdersDTO:Parse(): orders is not null. Try unmarshalling as a single order")
		} else {
			return nil, fmt.Errorf("TradierOrdersDTO:Parse(): failed to unmarshal orders: %w", err)
		}
	}

	// unmarshal orders object
	var payload TradierOrdersPayloadDTO
	if err := json.Unmarshal(dto.Orders, &payload); err != nil {
		return nil, fmt.Errorf("TradierOrdersDTO:Parse(): failed to unmarshal orders: %w", err)
	}

	// try to unmarshal as a single order
	var order TradierOrderDTO

	err = json.Unmarshal(payload.Order, &order)
	if err == nil {
		return []*TradierOrderDTO{&order}, nil
	} else {
		var unmarshalTypeError *json.UnmarshalTypeError
		if errors.As(err, &unmarshalTypeError) {
			// log.Debug("TradierOrdersDTO:Parse(): orders is not a single order. Try unmarshalling as a list of orders")
		} else {
			return nil, fmt.Errorf("TradierOrdersDTO:Parse(): failed to single unmarshal order: %w", err)
		}
	}

	// try to unmarshal as a list of orders
	var orders []*TradierOrderDTO

	if err := json.Unmarshal(payload.Order, &orders); err != nil {
		return nil, fmt.Errorf("TradierOrdersDTO:Parse(): failed to unmarshal orders: %w", err)
	}

	return orders, nil
}
