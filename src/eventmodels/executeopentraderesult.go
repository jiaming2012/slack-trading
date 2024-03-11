package eventmodels

import "github.com/google/uuid"

type ExecuteOpenTradeResult struct {
	Meta            *MetaData `json:"meta"`
	RequestID       uuid.UUID `json:"id"`
	PriceLevelIndex int       `json:"priceLevelIndex"`
	Trade           *Trade    `json:"trade"`
}

// type ExecuteOpenTradeResult struct {
// 	Meta      *MetaData               `json:"meta"`
// 	RequestID uuid.UUID               `json:"id"`
// 	Side      string                  `json:"side"`
// 	Result    *ExecuteOpenTradeResult `json:"result"`
// }

func (r *ExecuteOpenTradeResult) GetMetaData() *MetaData {
	return r.Meta
}

func (r *ExecuteOpenTradeResult) GetRequestID() uuid.UUID {
	return r.RequestID
}
