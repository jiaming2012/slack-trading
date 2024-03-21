package eventmodels

import "github.com/google/uuid"

type CreateSignalResponseEvent struct {
	Meta      *MetaData `json:"meta"`
	Name      string    `json:"name"`
	RequestID uuid.UUID `json:"requestID"`
}

func (r *CreateSignalResponseEvent) GetMetaData() *MetaData {
	return r.Meta
}

func (r *CreateSignalResponseEvent) GetRequestID() uuid.UUID {
	return r.RequestID
}
