package eventmodels

import "github.com/google/uuid"

type GetStatsResult struct {
	Meta       *MetaData             `json:"meta"`
	RequestID  uuid.UUID             `json:"requestID"`
	Strategies []*GetStatsResultItem `json:"strategies"`
}

func (r *GetStatsResult) GetMetaData() *MetaData {
	return r.Meta
}

func (r *GetStatsResult) GetRequestID() uuid.UUID {
	return r.RequestID
}
