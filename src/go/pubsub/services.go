package pubsub

import (
	"fmt"

	"github.com/asaskevich/EventBus"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

var bus EventBus.Bus

func Init() {
	bus = EventBus.New()
}

func PublishCompletedResponse(publisherName string, event RequestEvent, meta *models.MetaData) {
	event.SetMetaData(meta)
	publish(publisherName, models.ProcessRequestCompleteEventName, event)
}

func PublishResponse(publisherName string, topic models.EventName, event RequestEvent, meta *models.MetaData) {
	event.SetMetaData(meta)
	publish(publisherName, topic, event)
}

func PublishError(publisherName string, err error) {
	publishError(publisherName, err)
}

func PublishEvent(publisherName string, topic models.EventName, event interface{}) {
	publish(publisherName, topic, event)
}

func PublishAndSaveEvent(publisherName string, topic models.EventName, event models.SavedEvent) {
	PublishEvent(publisherName, topic, event)
}

func PublishRequestError(publisherName string, err error, meta *models.MetaData) {
	log.Error(err)

	terminalErr := models.NewTerminalError(meta, err)
	publish(publisherName, models.TerminalErrorName, terminalErr)
	publish("PublishEventError2", models.ProcessRequestCompleteEventName, terminalErr)
}

func publishError(publisherName string, err error) {
	log.Error(err)
	publish(publisherName, models.Error, err)
}

// Publish todo: only publish pointers to events
func publish(publisherName string, topic models.EventName, event interface{}) {
	publishWithFlags(publisherName, topic, event, true)
}

func publishWithFlags(publisherName string, topic models.EventName, event interface{}, logEvent bool) {
	var requestID uuid.UUID = uuid.Nil

	if reqEvent, ok := event.(RequestEvent); ok {
		requestID = reqEvent.GetMetaData().RequestID
	}

	if logEvent {
		var logMessage string
		if requestID != uuid.Nil {
			logMessage = fmt.Sprintf("[%v] Published to topic %s, using requestID %s", publisherName, topic, requestID.String())
		} else {
			logMessage = fmt.Sprintf("[%v] Published to topic %s. No request id was set.", publisherName, topic)
		}

		log.Debugf(logMessage)
	}

	bus.Publish(string(topic), event)
}

func Subscribe(subscriberName string, topic models.EventName, callbackFn interface{}) {
	if err := bus.SubscribeAsync(string(topic), callbackFn, false); err != nil {
		log.Errorf("[%v] error: %v", subscriberName, err)
	}

	log.Infof("[%v] Subscribed to topic %s", subscriberName, topic)
}

func Unsubscribe(subscriberName string, topic models.EventName, handler interface{}) error {
	err := bus.Unsubscribe(string(topic), handler)

	if err == nil {
		log.Infof("[%v] Unsubscribed from topic %s", subscriberName, topic)
	}

	return err
}
