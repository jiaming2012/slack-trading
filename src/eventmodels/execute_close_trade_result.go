package eventmodels

import "github.com/google/uuid"

type ExecuteCloseTradesResult struct {
	Meta      *MetaData `json:"meta"`
	RequestID uuid.UUID `json:"id"`
	Trade     *Trade    `json:"trade"`
}

// type ExecuteCloseTradesResult struct {
// 	Meta      *MetaData               `json:"meta"`
// 	RequestID uuid.UUID               `json:"id"`
// 	Side      string                  `json:"side"`
// 	Result    *ExecuteOpenTradeResult `json:"result"`
// }

func (r *ExecuteCloseTradesResult) GetMetaData() *MetaData {
	return r.Meta
}

func (r *ExecuteCloseTradesResult) GetRequestID() uuid.UUID {
	return r.RequestID
}
