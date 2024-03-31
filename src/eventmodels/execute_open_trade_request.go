package eventmodels

type ExecuteOpenTradeRequest struct {
	BaseRequestEvent2
	OpenTradeRequest *CreateTradeRequest
}
