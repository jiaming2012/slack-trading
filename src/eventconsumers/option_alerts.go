package eventconsumers

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventpubsub"
)

type OptionAlertWorker struct {
	wg           *sync.WaitGroup
	optionAlerts []*eventmodels.OptionAlert
}

func (w *OptionAlertWorker) handleGetOptionAlertRequestEvent(event *eventmodels.GetOptionAlertRequestEvent) {
	log.Debugf("OptionAlertWorker.handleGetOptionAlertRequestEvent: %v", event)

	currentAlerts := []eventmodels.OptionAlert{}

	for _, alert := range w.optionAlerts {
		currentAlerts = append(currentAlerts, *alert)
	}

	eventpubsub.PublishResult("OptionAlertWorker", eventpubsub.GetOptionAlertResponseEvent, &eventmodels.GetOptionAlertResponseEvent{
		RequestID: event.GetRequestID(),
		Alerts:    currentAlerts,
	})
}

func (w *OptionAlertWorker) handleCreateOptionAlertRequestEvent(event *eventmodels.CreateOptionAlertRequestEvent) {
	log.Debugf("OptionAlertWorker.handleCreateOptionAlertRequestEvent: %v", event)

	optionAlert, err := event.Convert()
	if err != nil {
		eventpubsub.PublishRequestError("OptionAlertWorker", event, err)
		return
	}

	w.optionAlerts = append(w.optionAlerts, optionAlert)

	eventpubsub.PublishResult("OptionAlertWorker", eventpubsub.CreateOptionAlertResponseEvent, &eventmodels.CreateOptionAlertResponseEvent{
		BaseResponseEvent: eventmodels.BaseResponseEvent{
			RequestID: event.GetRequestID(),
		},
		ID: optionAlert.ID.String(),
	})
}

func (w *OptionAlertWorker) handleDeleteOptionAlertRequestEvent(event *eventmodels.DeleteOptionAlertRequestEvent) {
	log.Debugf("OptionAlertWorker.handleDeleteOptionAlertRequestEvent: %v", event)

	for i, alert := range w.optionAlerts {
		if alert.ID == event.AlertID {
			w.optionAlerts = append(w.optionAlerts[:i], w.optionAlerts[i+1:]...)
			break
		}
	}

	eventpubsub.PublishResult("OptionAlertWorker", eventpubsub.DeleteOptionAlertResponseEvent, &eventmodels.DeleteOptionAlertResponseEvent{
		BaseResponseEvent: eventmodels.BaseResponseEvent{
			RequestID: event.GetRequestID(),
		},
	})
}

func (w *OptionAlertWorker) Start(ctx context.Context) {
	w.wg.Add(1)

	eventpubsub.Subscribe("OptionAlertWorker", eventpubsub.GetOptionAlertRequestEvent, w.handleGetOptionAlertRequestEvent)
	eventpubsub.Subscribe("OptionAlertWorker", eventpubsub.CreateOptionAlertRequestEvent, w.handleCreateOptionAlertRequestEvent)
	eventpubsub.Subscribe("OptionAlertWorker", eventpubsub.DeleteOptionAlertRequestEvent, w.handleDeleteOptionAlertRequestEvent)

	go func() {
		defer w.wg.Done()

		for {
			select {
			case <-ctx.Done():
				log.Info("stopping OptionAlertWorker consumer")
				return
			}
		}
	}()
}

func NewOptionAlertWorker(wg *sync.WaitGroup) *OptionAlertWorker {
	return &OptionAlertWorker{
		wg:           wg,
		optionAlerts: []*eventmodels.OptionAlert{},
	}
}
