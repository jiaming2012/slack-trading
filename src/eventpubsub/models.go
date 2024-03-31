package eventpubsub

import (
	"slack-trading/src/eventmodels"
)

type RequestEvent interface {
	GetMetaData() eventmodels.MetaData
	SetMetaData(*eventmodels.MetaData)
}

type SagaFlow struct {
	Generate func() RequestEvent
}
