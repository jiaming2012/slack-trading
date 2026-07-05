package models

import (

	"github.com/google/uuid"
)

type AddAccountResponseEvent struct {
	RequestID uuid.UUID
	Account   *Account
}

func (e *AddAccountResponseEvent) GetRequestID() uuid.UUID {
	return e.RequestID
}
