package eventmodels

import "github.com/google/uuid"

type RequestHeader struct {
	Meta      *MetaData `json:"meta"`
	RequestID uuid.UUID `json:"requestID"`
}

func (r *RequestHeader) GetMetaData() *MetaData {
	return r.Meta
}

func (r *RequestHeader) GetRequestID() uuid.UUID {
	return r.RequestID
}

type AccountsRequestHeader struct {
	RequestHeader
	AccountName string `json:"accountName"`
}
