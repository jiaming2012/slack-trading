package eventmodels

import "github.com/google/uuid"

type NewSignalResult struct {
	Name      string    `json:"name"`
	RequestID uuid.UUID `json:"requestID"`
}

func (r *NewSignalResult) GetRequestID() uuid.UUID {
	return r.RequestID
}
