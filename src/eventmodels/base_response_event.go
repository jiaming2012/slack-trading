package eventmodels

import "github.com/google/uuid"

type BaseResponseEvent struct {
	Meta      *MetaData `json:"meta"`
	RequestID uuid.UUID `json:"id"`
}

func (r *BaseResponseEvent) GetMetaData() *MetaData {
	return r.Meta
}

func (r *BaseResponseEvent) GetRequestID() uuid.UUID {
	return r.RequestID
}

func (r *BaseResponseEvent) SetMetaData(meta *MetaData) {
	r.Meta = meta
}

func (r *BaseResponseEvent) SetRequestID(requestID uuid.UUID) {
	r.RequestID = requestID
}
