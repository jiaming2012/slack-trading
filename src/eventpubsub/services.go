package eventpubsub

import (
	"fmt"

	"github.com/asaskevich/EventBus"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
)

var bus EventBus.Bus

func Init() {
	bus = EventBus.New()
}

func PublishCompletedResponse(publisherName string, event RequestEvent, meta *eventmodels.MetaData) {
	event.SetMetaData(meta)
	publish(publisherName, eventmodels.ProcessRequestCompleteEventName, event)
}

func PublishResponse(publisherName string, topic eventmodels.EventName, event RequestEvent, meta *eventmodels.MetaData) {
	event.SetMetaData(meta)
	publish(publisherName, topic, event)
}

func PublishError(publisherName string, err error) {
	publishError(publisherName, err)
}

func PublishEventResultDeprecated(publisherName string, topic eventmodels.EventName, event interface{}) {
	publish(publisherName, topic, event)
}

func PublishRequestError(publisherName string, err error, meta *eventmodels.MetaData) {
	log.Error(err)

	terminalErr := eventmodels.NewTerminalError(meta, err)
	publish(publisherName, eventmodels.TerminalErrorName, terminalErr)
	publish("PublishEventError2", eventmodels.ProcessRequestCompleteEventName, terminalErr)
}

func publishError(publisherName string, err error) {
	log.Error(err)
	publish(publisherName, eventmodels.Error, err)
}

// Publish todo: only publish pointers to events
func publish(publisherName string, topic eventmodels.EventName, event interface{}) {
	publishWithFlags(publisherName, topic, event, true)
}

func publishWithFlags(publisherName string, topic eventmodels.EventName, event interface{}, logEvent bool) {
	var requestID *uuid.UUID

	if reqEvent, ok := event.(RequestEvent); ok {
		requestID = &reqEvent.GetMetaData().RequestID
	}

	if logEvent {
		var logMessage string
		if requestID != nil {
			logMessage = fmt.Sprintf("[%v] Published to topic %s, using requestID %s", publisherName, topic, requestID.String())
		} else {
			logMessage = fmt.Sprintf("[%v] Published to topic %s. No request id was set.", publisherName, topic)
		}

		log.Debugf(logMessage)
	}

	bus.Publish(string(topic), event)
}

func Subscribe(subscriberName string, topic eventmodels.EventName, callbackFn interface{}) {
	if err := bus.SubscribeAsync(string(topic), callbackFn, false); err != nil {
		log.Errorf("[%v] error: %v", subscriberName, err)
	}

	log.Infof("[%v] Subscribed to topic %s", subscriberName, topic)
}
