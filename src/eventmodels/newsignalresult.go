package eventmodels

import "github.com/google/uuid"

type CreateSignalResultEvent struct {
	Meta      *MetaData `json:"meta"`
	Name      string    `json:"name"`
	RequestID uuid.UUID `json:"requestID"`
}

func (r *CreateSignalResultEvent) GetMetaData() *MetaData {
	return r.Meta
}

func (r *CreateSignalResultEvent) GetRequestID() uuid.UUID {
	return r.RequestID
}
