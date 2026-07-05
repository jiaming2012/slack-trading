package eventpubsub

import "github.com/jiaming2012/slack-trading/src/go/models"

type RequestEvent interface {
	GetMetaData() *models.MetaData
	SetMetaData(*models.MetaData)
}

type SagaFlow struct {
	Generate func() RequestEvent
}

func NewSagaFlow() map[models.EventName]SagaFlow {
	return map[models.EventName]SagaFlow{
		models.CreateAccountRequestEventName: {
			Generate: func() RequestEvent { return &models.CreateAccountRequestEventV1{} },
		},
		models.CreateAccountStrategyRequestEventName: {
			Generate: func() RequestEvent { return &models.CreateAccountStrategyRequestEvent{} },
		},
		models.CreateSignalRequestEventName: {
			Generate: func() RequestEvent { return &models.CreateSignalRequestEventV1DTO{} },
		},
		models.CreateOptionAlertRequestEventName: {
			Generate: func() RequestEvent { return &models.CreateOptionAlertRequestEvent{} },
		},
		models.DeleteOptionAlertRequestEventName: {
			Generate: func() RequestEvent { return &models.DeleteOptionAlertRequestEvent{} },
		},
		models.OptionAlertUpdateEventName: {
			Generate: func() RequestEvent { return &models.OptionAlertUpdateEvent{} },
		},
		models.CreateNewOptionChainTickEvent: {
			Generate: func() RequestEvent { return &models.OptionChainTickV1{} },
		},
		models.CreateNewStockTickEvent: {
			Generate: func() RequestEvent { return &models.StockTickV1{} },
		},
		models.CreateOptionContractEvent: {
			Generate: func() RequestEvent { return &models.OptionContractV1{} },
		},
	}
}
