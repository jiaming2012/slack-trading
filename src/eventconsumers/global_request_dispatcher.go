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

	// fixed: Too many places to add
	case *eventmodels.OptionAlertUpdateCompletedEvent:
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

	pubsub.Subscribe("GlobalDispatchWorker", eventmodels.Error, w.dispatchError)
	pubsub.Subscribe("GlobalDispatchWorker", eventmodels.ExecuteOpenTradeResultEventName, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", eventmodels.FetchTradesResultEventName, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", eventmodels.ExecuteCloseTradesResultEventName, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", eventmodels.GetStatsResultEventName, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", eventmodels.CreateSignalResponseEventName, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", eventmodels.ManualDatafeedUpdateResultEventName, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", eventmodels.GetAccountsResponseEventName, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", eventmodels.CreateAccountResponseEventName, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", eventmodels.CreateStrategyResponseEventName, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", eventmodels.ProcessRequestCompleteEventName, w.dispatchResult)

	// fixed: too many places to add
	pubsub.Subscribe("GlobalDispatchWorker", eventmodels.OptionAlertUpdateCompletedEventName, w.dispatchResult)

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
