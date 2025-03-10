package models

import (
	"context"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type IBroker interface {
	PlaceOrder(ctx context.Context, req *PlaceEquityTradeRequest) (map[string]interface{}, error)
	FetchOrders(ctx context.Context) ([]*eventmodels.TradierOrder, error)
	FetchQuotes(ctx context.Context, symbols []eventmodels.Instrument) ([]*TradierQuoteDTO, error)
	FetchOrder(orderID uint, liveAccountType LiveAccountType) (*eventmodels.TradierOrder, error)
	FetchBalances(url, token string) (eventmodels.FetchTradierBalancesResponseDTO, error)
	GetSource() ILiveAccountSource
	FetchEquity() (*eventmodels.FetchAccountEquityResponse, error)
}
