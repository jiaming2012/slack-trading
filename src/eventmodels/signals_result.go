package eventmodels

import "github.com/google/uuid"

type NewSignalResult struct {
	RequestID          uuid.UUID `json:"requestID"`
	StrategiesAffected int       `json:"strategiesAffected"`
}

func (r *NewSignalResult) GetRequestID() uuid.UUID {
	return r.RequestID
}
