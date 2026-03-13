package eventmodels

type ExecuteCloseTradeRequest struct {
	BaseRequestEvent
	Timeframe *int
	Trade     *Trade
	Percent   float64
}

type ExecuteCloseTradesRequest struct {
	BaseRequestEvent
	CloseTradesRequest *CloseTradesRequest
}
