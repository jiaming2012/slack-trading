package eventconsumers

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	pubsub "github.com/jiaming2012/slack-trading/src/eventpubsub"
)

type GlobalDispatchWorker struct {
	wg         *sync.WaitGroup
	dispatcher *eventmodels.GlobalResponseDispatcher
}

func (w *GlobalDispatchWorker) dispatchResult(event eventmodels.ResultEvent) {
	meta := event.GetMetaData()
	if !meta.IsExternalRequest {
		return
	}

	id := meta.RequestID
	globalDispatchItem, err := w.dispatcher.GetChannelAndRemove(id)
	if err != nil {
		log.Debugf("GlobalDispatchWorker.dispatchResult: failed to find dispatcher: %v", err)
		return
	}

	// return external api response
	switch ev := event.(type) {
	case *eventmodels.TerminalError:
		globalDispatchItem.ErrCh <- ev.Error
	default:
		globalDispatchItem.ResultCh <- event
	}
}

func (w *GlobalDispatchWorker) Start(ctx context.Context) {
	w.wg.Add(1)

	pubsub.Subscribe("GlobalDispatchWorker", eventmodels.FetchTradesResultEventName, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", eventmodels.ExecuteCloseTradesResultEventName, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", eventmodels.ProcessRequestCompleteEventName, w.dispatchResult)

	// todo: fix: too many places to add

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
