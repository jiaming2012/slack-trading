package eventmodels

import "slack-trading/src/models"

type ExecuteCloseTradeRequest struct {
	PriceLevel         *models.PriceLevel
	CloseTradesRequest models.CloseTradesRequest
}
