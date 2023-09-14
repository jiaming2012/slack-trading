package eventpubsub

import (
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

func Publish(publisherName string, topic EventName, event interface{}) {
	PublishWithFlags(publisherName, topic, event, true)
}

func PublishWithFlags(publisherName string, topic EventName, event interface{}, logEvent bool) {
	if logEvent {
		log.Debugf("[%v] Published to topic %s", publisherName, topic)
	}

	bus.Publish(string(topic), event)
}

func Subscribe(subscriberName string, topic EventName, callbackFn interface{}) error {
	if err := bus.SubscribeAsync(string(topic), callbackFn, false); err != nil {
		return err
	}

	log.Infof("[%v] Subscribed to topic %s", subscriberName, topic)
	return nil
}
