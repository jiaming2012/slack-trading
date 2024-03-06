package eventpubsub

import (
	"fmt"

	"github.com/asaskevich/EventBus"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
)

var bus EventBus.Bus

func Init() {
	bus = EventBus.New()
}

func PublishRequestError[T eventmodels.RequestEvent](publisherName string, requestEvent T, err error) {
	requestErr := eventmodels.NewRequestError(requestEvent.GetRequestID(), err)
	PublishError(publisherName, requestErr)
}

func PublishError(publisherName string, err error) {
	log.Error(err)
	Publish(publisherName, Error, err)
}

func PublishResult[T eventmodels.ResultEvent](publisherName string, topic EventName, event T) {
	PublishWithFlags(publisherName, topic, event, true)
}

// Publish todo: only publish pointers to events
func Publish(publisherName string, topic EventName, event interface{}) {
	PublishWithFlags(publisherName, topic, event, true)
}

func PublishWithFlags(publisherName string, topic EventName, event interface{}, logEvent bool) {
	var requestID *string
	switch ev := event.(type) {
	case eventmodels.RequestEvent:
		_id := ev.GetRequestID().String()
		requestID = &_id
	}

	if logEvent {
		var logMessage string
		if requestID != nil {
			logMessage = fmt.Sprintf("[%v] Published to topic %s, using requestID %s", publisherName, topic, *requestID)
		} else {
			logMessage = fmt.Sprintf("[%v] Published to topic %s", publisherName, topic)
		}

		log.Debugf(logMessage)
	}

	bus.Publish(string(ProcessRequestComplete), event)

	bus.Publish(string(topic), event)
}

func Subscribe(subscriberName string, topic EventName, callbackFn interface{}) error {
	if err := bus.SubscribeAsync(string(topic), callbackFn, false); err != nil {
		return err
	}

	log.Infof("[%v] Subscribed to topic %s", subscriberName, topic)
	return nil
}
