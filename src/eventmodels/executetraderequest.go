package eventmodels

import "slack-trading/src/models"

type ExecuteCloseTradeRequest struct {
	PriceLevel         *models.PriceLevel
	CloseTradesRequest models.CloseTradesRequest
}

type ExecuteOpenTradeRequest struct {
	OpenTradeRequest *models.OpenTradeRequest
	Result           chan *ExecuteOpenTradeResult
	Error            chan error
}
