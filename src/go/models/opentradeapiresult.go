package models

type OpenTradeApiResult struct {
	RequestID              string                  `json:"id"`
	ExecuteOpenTradeResult *ExecuteOpenTradeResult `json:"result"`
}
