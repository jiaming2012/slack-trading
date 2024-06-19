package eventmodels

import (
	"github.com/jiaming2012/slack-trading/src/models"

	"github.com/google/uuid"
)

type AddAccountResponseEvent struct {
	RequestID uuid.UUID
	Account   *models.Account
}

func (e *AddAccountResponseEvent) GetRequestID() uuid.UUID {
	return e.RequestID
}
