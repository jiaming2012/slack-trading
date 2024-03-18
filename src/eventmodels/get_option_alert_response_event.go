package eventmodels

import "github.com/google/uuid"

type GetOptionAlertResponseEvent struct {
	Meta      *MetaData     `json:"meta"`
	RequestID uuid.UUID     `json:"id"`
	Alerts    []OptionAlert `json:"alerts"`
}

func (r *GetOptionAlertResponseEvent) GetMetaData() *MetaData {
	return r.Meta
}

func (r *GetOptionAlertResponseEvent) GetRequestID() uuid.UUID {
	return r.RequestID
}
