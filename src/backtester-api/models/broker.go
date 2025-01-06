package models

import "context"

type IBroker interface {
	PlaceOrder(ctx context.Context, req *PlaceEquityTradeRequest) error
}
