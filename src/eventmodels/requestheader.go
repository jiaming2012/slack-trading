package eventmodels

import "github.com/google/uuid"

type RequestHeader struct {
	RequestID uuid.UUID `json:"requestID"`
}

func (r *RequestHeader) GetRequestID() uuid.UUID {
	return r.RequestID
}

type AccountsRequestHeader struct {
	RequestHeader
	AccountName string `json:"accountName"`
}
