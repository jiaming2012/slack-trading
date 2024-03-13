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

func PublishRequestErrorInterface(publisherName string, requestEvent eventmodels.RequestEvent, err error) {
	switch ev := requestEvent.(type) {
	case eventmodels.ExecuteOpenTradeRequest:
		fmt.Println(ev)
		openTradeReq := ev.ParentRequest.(*eventmodels.OpenTradeRequest)
		openTradeReq.Error <- eventmodels.EventError{
			Request: ev,
			Error:   err,
		}
	}

	requestErr := eventmodels.NewRequestError(requestEvent.GetRequestID(), err)
	publishError(publisherName, requestErr)
}

func PublishRequestError[T eventmodels.RequestEvent](publisherName string, requestEvent T, err error) {
	requestErr := eventmodels.NewRequestError(requestEvent.GetRequestID(), err)
	publishError(publisherName, requestErr)
}

func PublishResultWithMetadata[T eventmodels.ResultEvent](publisherName string, topic EventName, event T, meta *eventmodels.MetaData) {
	publish(publisherName, topic, event)
}

func PublishResult[T eventmodels.ResultEvent](publisherName string, topic EventName, event T) {
	publish(publisherName, topic, event)
}

func PublishEventError(publisherName string, err error) {
	publishError(publisherName, err)
}

func PublishEventResult(publisherName string, topic EventName, event interface{}) {
	publish(publisherName, topic, event)
}

func publishError(publisherName string, err error) {
	log.Error(err)
	publish(publisherName, Error, err)
}

// Publish todo: only publish pointers to events
func publish(publisherName string, topic EventName, event interface{}) {
	publishWithFlags(publisherName, topic, event, true)
}

// APIRequestEvent (creates)-> StreamWriteEvent (creates)-> *StreamWriteProcessedEvent (creates)-> StreamReadEvent (creates)-> **StreamReadProcessedEvent -> APISuccessEvent
// *StreamWriteErrorEvent (creates)-> APIErrorEvent | APIRequestLookup(success)
// **StreamReadErrorEvent (creates)-> APIErrorEvent | APIRequestLookup(success)
func publishWithFlags(publisherName string, topic EventName, event interface{}, logEvent bool) {
	var requestID *string
	switch ev := event.(type) {
	// case eventmodels.APIRequestEvent:
	// case eventmodels.StreamReadEvent:
	case eventmodels.RequestError:
		fmt.Println(ev)
	case eventmodels.RequestEvent:
		_id := ev.GetRequestID().String()
		requestID = &_id

		// if !strings.Contains(strings.ToLower(string(topic)), "error") {

		// }
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

	bus.Publish(string(topic), event)
}

func Subscribe(subscriberName string, topic EventName, callbackFn interface{}) error {
	if err := bus.SubscribeAsync(string(topic), callbackFn, false); err != nil {
		return err
	}

	log.Infof("[%v] Subscribed to topic %s", subscriberName, topic)
	return nil
}
