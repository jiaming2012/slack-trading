package eventpubsub

import (
	"slack-trading/src/eventmodels"
)

type TerminalRequest interface {
	GetMetaData() *eventmodels.MetaData
	SetMetaData(*eventmodels.MetaData)
}

type SagaFlow struct {
	Generate  func() TerminalRequest
	NextEvent eventmodels.EventName
}
