package models

import (
	"github.com/jiaming2012/slack-trading/src/go/eventdto"
)

type AddStrategyRequest struct {
	eventdto.Header
	Price     float64
	Direction Direction
}

func NewAddStrategyRequest(header eventdto.Header, direction Direction, price float64) *AddStrategyRequest {
	return &AddStrategyRequest{
		Header:    header,
		Direction: direction,
		Price:     price,
	}
}
