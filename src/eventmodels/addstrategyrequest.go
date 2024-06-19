package eventmodels

import (
	"github.com/jiaming2012/slack-trading/src/eventdto"
	"github.com/jiaming2012/slack-trading/src/models"
)

type AddStrategyRequest struct {
	eventdto.Header
	Price     float64
	Direction models.Direction
}

func NewAddStrategyRequest(header eventdto.Header, direction models.Direction, price float64) *AddStrategyRequest {
	return &AddStrategyRequest{
		Header:    header,
		Direction: direction,
		Price:     price,
	}
}
