package eventmodels

import "github.com/google/uuid"

type BaseRequstEvent struct {
	Meta      *MetaData `json:"meta"`
	RequestID uuid.UUID `json:"id"`
}

func (r *BaseRequstEvent) GetMetaData() *MetaData {
	return r.Meta
}

func (r *BaseRequstEvent) GetRequestID() uuid.UUID {
	return r.RequestID
}

func (r *BaseRequstEvent) SetMetaData(meta *MetaData) {
	r.Meta = meta
}

func (r *BaseRequstEvent) SetRequestID(requestID uuid.UUID) {
	r.RequestID = requestID
}
