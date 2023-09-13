package eventmodels

import "github.com/google/uuid"

type ResultEvent interface {
	GetRequestID() uuid.UUID
}
