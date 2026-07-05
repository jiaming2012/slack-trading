package models

import (
	"context"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

type IBroker interface {
	PlaceOrder(ctx context.Context, req *PlaceOrderRequest) (map[string]interface{}, error)
	FetchOrders(ctx context.Context) ([]*models.TradierOrder, error)
	FetchQuotes(ctx context.Context, symbols []models.Instrument) ([]*TradierQuoteDTO, error)
	FetchOrder(orderID uint, liveAccountType LiveAccountType) (*models.TradierOrder, error)
	FetchBalances(url, token string) (models.FetchTradierBalancesResponseDTO, error)
	GetSource() ILiveAccountSource
	FetchEquity() (*models.FetchAccountEquityResponse, error)
	FetchPositions() ([]models.TradierPositionDTO, error)
	FillOrder(orderId uint, price float64, status string) error
}
