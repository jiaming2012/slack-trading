package eventpubsub

import (
	"github.com/asaskevich/EventBus"
	log "github.com/sirupsen/logrus"
)

var bus EventBus.Bus

func Init() {
	bus = EventBus.New()
}

func PublishError(publisherName string, err error) {
	log.Error(err)
	Publish(publisherName, Error, err)
}

func Publish(publisherName string, topic string, event interface{}) {
	PublishWithFlags(publisherName, topic, event, true)
}

func PublishWithFlags(publisherName string, topic string, event interface{}, logEvent bool) {
	if logEvent {
		log.Debugf("[%v] Published to topic %s", publisherName, topic)
	}

	bus.Publish(topic, event)
}

func Subscribe(subscriberName string, topic string, callbackFn interface{}) error {
	if err := bus.SubscribeAsync(topic, callbackFn, false); err != nil {
		return err
	}

	log.Infof("[%v] Subscribed to topic %s", subscriberName, topic)
	return nil
}
