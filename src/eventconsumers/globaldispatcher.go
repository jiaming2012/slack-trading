package eventconsumers

import (
	"context"
	"fmt"
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
		pubsub.PublishError("GlobalDispatchWorker.dispatchError", fmt.Errorf("failed to find dispatcher: %w", err))
		return
	}

	globalDispatchItem.ErrCh <- requestErr
}

func (w *GlobalDispatchWorker) dispatchResult(event eventmodels.ResultEvent) {
	// todo: when the request is originated from the db, the requestID is not set
	id := event.GetRequestID()
	globalDispatchItem, found := w.dispatcher.Channels[id]
	// pubsub.Publish("GlobalDispatchWorker.dispatchResult", pubsub.RequestCompletedEvent, id)

	if !found {
		// pubsub.PublishRequestError("GlobalDispatchWorker.dispatchResult", event, fmt.Errorf("failed to find dispatcher, using requestID %v", id))
		log.Errorf("dispatchResult: failed to find dispatcher, using requestID %v", id)
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
	pubsub.Subscribe("GlobalDispatchWorker", pubsub.NewSignalResultEvent, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", pubsub.ManualDatafeedUpdateResult, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", pubsub.GetAccountsResponseEvent, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", pubsub.CreateAccountResponseEvent, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", pubsub.CreateStrategyResponseEvent, w.dispatchResult)

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
