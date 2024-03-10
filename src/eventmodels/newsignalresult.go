package eventmodels

import "github.com/google/uuid"

type NewSignalResult struct {
	Meta      *MetaData `json:"meta"`
	Name      string    `json:"name"`
	RequestID uuid.UUID `json:"requestID"`
}

func (r *NewSignalResult) GetMetaData() *MetaData {
	return r.Meta
}

func (r *NewSignalResult) GetRequestID() uuid.UUID {
	return r.RequestID
}
