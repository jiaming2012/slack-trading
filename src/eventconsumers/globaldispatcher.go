package eventconsumers

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	"sync"
)

type GlobalDispatchWorker struct {
	wg         *sync.WaitGroup
	dispatcher *eventmodels.GlobalResponseDispatcher
	mu         sync.Mutex // todo: maybe make use of this mutext ??
}

func (w *GlobalDispatchWorker) dispatchError(err error) {
	requestErr, ok := err.(eventmodels.RequestError)
	if !ok {
		return
	}

	uuid := requestErr.RequestID()
	globalDispatchItem, err := w.dispatcher.GetChannelAndRemove(uuid)
	if err != nil {
		pubsub.PublishError("GlobalDispatchWorker.dispatchError", fmt.Errorf("failed to find dispatcher: %w", err))
		return
	}

	globalDispatchItem.ErrCh <- requestErr
}

func (w *GlobalDispatchWorker) dispatchResult(event eventmodels.ResultEvent) {
	uuid := event.GetRequestID()
	globalDispatchItem, found := w.dispatcher.Channels[uuid]
	if !found {
		pubsub.PublishError("GlobalDispatchWorker.dispatchError", fmt.Errorf("failed to find dispatcher, using requestID %v", uuid))
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
