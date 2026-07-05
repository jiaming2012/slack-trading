package models

type ExecuteOpenTradeRequest struct {
	BaseRequestEvent
	OpenTradeRequest *CreateTradeRequest
}
