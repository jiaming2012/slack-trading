package eventpubsub

import "github.com/jiaming2012/slack-trading/src/eventmodels"

type RequestEvent interface {
	GetMetaData() *eventmodels.MetaData
	SetMetaData(*eventmodels.MetaData)
}

type SagaFlow struct {
	Generate func() RequestEvent
}

func NewSagaFlow() map[eventmodels.EventName]SagaFlow {
	return map[eventmodels.EventName]SagaFlow{
		eventmodels.CreateAccountRequestEventName: {
			Generate: func() RequestEvent { return &eventmodels.CreateAccountRequestEventV1{} },
		},
		eventmodels.CreateAccountStrategyRequestEventName: {
			Generate: func() RequestEvent { return &eventmodels.CreateAccountStrategyRequestEvent{} },
		},
		eventmodels.CreateSignalRequestEventName: {
			Generate: func() RequestEvent { return &eventmodels.CreateSignalRequestEventV1DTO{} },
		},
		eventmodels.CreateOptionAlertRequestEventName: {
			Generate: func() RequestEvent { return &eventmodels.CreateOptionAlertRequestEvent{} },
		},
		eventmodels.DeleteOptionAlertRequestEventName: {
			Generate: func() RequestEvent { return &eventmodels.DeleteOptionAlertRequestEvent{} },
		},
		eventmodels.OptionAlertUpdateEventName: {
			Generate: func() RequestEvent { return &eventmodels.OptionAlertUpdateEvent{} },
		},
		eventmodels.CreateNewOptionChainTickEvent: {
			Generate: func() RequestEvent { return &eventmodels.OptionChainTickV1{} },
		},
		eventmodels.CreateNewStockTickEvent: {
			Generate: func() RequestEvent { return &eventmodels.StockTickV1{} },
		},
		eventmodels.CreateOptionContractEvent: {
			Generate: func() RequestEvent { return &eventmodels.OptionContractV1{} },
		},
	}
}
