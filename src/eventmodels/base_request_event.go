package eventmodels

import "github.com/google/uuid"

type BaseRequestEvent2 struct {
	Meta *MetaData `json:"meta"`
}

func (r *BaseRequestEvent2) GetMetaData() *MetaData {
	return r.Meta
}

func (r *BaseRequestEvent2) SetMetaData(meta *MetaData) {
	r.Meta = meta
}

type BaseRequstEvent struct {
	Meta      *MetaData `json:"meta"`
	RequestID uuid.UUID `json:"requestID"`
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
