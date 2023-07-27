package eventpubsub

import (
	"github.com/asaskevich/EventBus"
	log "github.com/sirupsen/logrus"
)

var bus EventBus.Bus

func Init() {
	bus = EventBus.New()
}

func Publish(topic string, event interface{}) {
	bus.Publish(topic, event)
}

func Subscribe(topic string, callbackFn interface{}) error {
	if err := bus.SubscribeAsync(topic, callbackFn, false); err != nil {
		return err
	}

	log.Infof("Subscribed to topic %s", topic)
	return nil
}
