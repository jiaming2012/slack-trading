package eventmodels

import "github.com/google/uuid"

type ResultEvent interface {
	GetMetaData() *MetaData
	GetRequestID() uuid.UUID
}
