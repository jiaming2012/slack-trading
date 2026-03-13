package eventmodels

import "github.com/google/uuid"

type RequestEvent interface {
	GetRequestID() uuid.UUID
}
