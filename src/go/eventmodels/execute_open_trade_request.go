package eventmodels

type ExecuteOpenTradeRequest struct {
	BaseRequestEvent
	OpenTradeRequest *CreateTradeRequest
}
