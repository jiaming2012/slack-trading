package eventconsumers

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
)

type GlobalDispatchWorker struct {
	wg         *sync.WaitGroup
	dispatcher *eventmodels.GlobalResponseDispatcher
}

func (w *GlobalDispatchWorker) dispatchError(err error) {
	requestErr, ok := err.(eventmodels.RequestError)
	if !ok {
		// pubsub.Publish("GlobalDispatchWorker.dispatchError", pubsub.RequestCompletedEvent, uuid.Nil)
		log.Warn("dispatchError: failed to cast error to RequestError")
		return
	}

	id := requestErr.RequestID()

	// pubsub.Publish("GlobalDispatchWorker.dispatchError", pubsub.RequestCompletedEvent, id)

	globalDispatchItem, err := w.dispatcher.GetChannelAndRemove(id)
	if err != nil {
		// pubsub.PublishError("GlobalDispatchWorker.dispatchError", fmt.Errorf("failed to find dispatcher: %w", err))
		log.Debugf("GlobalDispatchWorker.dispatchError: failed to find dispatcher: %v", err)
		return
	}

	globalDispatchItem.ErrCh <- requestErr
}

func (w *GlobalDispatchWorker) dispatchResult(event eventmodels.ResultEvent) {
	switch event.(type) {
	// case *eventmodels.CreateAccountResponseEvent:
	// 	eventpubsub.PublishEventResult("GlobalDispatchWorker", pubsub.ProcessRequestComplete, ev)
	// case *eventmodels.CreateAccountStrategyResponseEvent:
	// 	eventpubsub.PublishEventResult("GlobalDispatchWorker", pubsub.ProcessRequestComplete, ev)
	// case *eventmodels.CreateSignalResponseEvent:
	// 	eventpubsub.PublishEventResult("GlobalDispatchWorker", pubsub.ProcessRequestComplete, ev)
	// case *eventmodels.CreateOptionAlertResponseEvent:
	// 	eventpubsub.PublishEventResult("GlobalDispatchWorker", pubsub.ProcessRequestComplete, ev)
	// case *eventmodels.DeleteOptionAlertResponseEvent:
	// 	eventpubsub.PublishEventResult("GlobalDispatchWorker", pubsub.ProcessRequestComplete, ev)
	// Too many places to add
	case *eventmodels.OptionAlertUpdateCompletedEvent:
		// eventpubsub.PublishEventResult("GlobalDispatchWorker", pubsub.ProcessRequestComplete, ev)
		// return as this cannot originate from an external request
		return
	}

	// todo: when the request is originated from the db, the requestID is not set. I THINK THIS IS FIXED!
	id := event.GetMetaData().RequestID
	globalDispatchItem, err := w.dispatcher.GetChannelAndRemove(id)

	if err != nil {
		log.Debugf("GlobalDispatchWorker.dispatchResult: failed to find dispatcher: %v", err)
		return
	}

	globalDispatchItem.ResultCh <- event
}

func (w *GlobalDispatchWorker) Start(ctx context.Context) {
	w.wg.Add(1)

	pubsub.Subscribe("GlobalDispatchWorker", pubsub.Error, w.dispatchError)
	pubsub.Subscribe("GlobalDispatchWorker", pubsub.ExecuteOpenTradeResult, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", pubsub.FetchTradesResult, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", pubsub.ExecuteCloseTradesResult, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", pubsub.GetStatsResult, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", pubsub.CreateSignalResponseEvent, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", pubsub.ManualDatafeedUpdateResult, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", pubsub.GetAccountsResponseEvent, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", pubsub.CreateAccountResponseEvent, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", pubsub.CreateStrategyResponseEvent, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", pubsub.ProcessRequestComplete, w.dispatchResult)

	// fixed: too many places to add
	pubsub.Subscribe("GlobalDispatchWorker", pubsub.OptionAlertUpdateCompletedEvent, w.dispatchResult)

	go func() {
		defer w.wg.Done()
		for {
			select {
			case <-ctx.Done():
				log.Info("stopping CandleWorker consumer")
				return
			}
		}
	}()
}

func NewGlobalDispatcherWorkerClient(wg *sync.WaitGroup, dispatcher *eventmodels.GlobalResponseDispatcher) *GlobalDispatchWorker {
	return &GlobalDispatchWorker{
		wg:         wg,
		dispatcher: dispatcher,
	}
}
