package eventconsumers

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/models"
	pubsub "github.com/jiaming2012/slack-trading/src/go/eventpubsub"
)

type GlobalDispatchWorker struct {
	wg         *sync.WaitGroup
	dispatcher *models.GlobalResponseDispatcher
}

func (w *GlobalDispatchWorker) dispatchResult(event models.ResultEvent) {
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
	case *models.TerminalError:
		globalDispatchItem.ErrCh <- ev.Error
	default:
		globalDispatchItem.ResultCh <- event
	}
}

func (w *GlobalDispatchWorker) Start(ctx context.Context) {
	w.wg.Add(1)

	pubsub.Subscribe("GlobalDispatchWorker", models.FetchTradesResultEventName, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", models.ExecuteCloseTradesResultEventName, w.dispatchResult)
	pubsub.Subscribe("GlobalDispatchWorker", models.ProcessRequestCompleteEventName, w.dispatchResult)

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

func NewGlobalDispatcherWorkerClient(wg *sync.WaitGroup, dispatcher *models.GlobalResponseDispatcher) *GlobalDispatchWorker {
	return &GlobalDispatchWorker{
		wg:         wg,
		dispatcher: dispatcher,
	}
}
