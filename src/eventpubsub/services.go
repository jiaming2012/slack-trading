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
		openTradeReq := ev.ParentRequest.(*eventmodels.CreateTradeRequest)
		openTradeReq.Error <- eventmodels.RequestError2{
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

func PublishResultWithMetadata[T eventmodels.ResultEvent](publisherName string, topic eventmodels.EventName, event T, meta *eventmodels.MetaData) {
	publish(publisherName, topic, event)
}

func PublishResult2(publisherName string, topic eventmodels.EventName, event interface{}) {
	publish(publisherName, topic, event)
}

func PublishResult3(publisherName string, event TerminalRequest, meta *eventmodels.MetaData) {
	event.SetMetaData(meta)
	publish(publisherName, eventmodels.ProcessRequestCompleteEventName, event)
}

func PublishResult[T eventmodels.ResultEvent](publisherName string, topic eventmodels.EventName, event T) {
	publish(publisherName, topic, event)
}

func PublishEventError(publisherName string, err error) {
	publishError(publisherName, err)
}

func PublishEventError2(publisherName string, meta *eventmodels.MetaData, err error) {
	publish("PublishEventError2", eventmodels.ProcessRequestCompleteEventName, &eventmodels.TerminalError{
		Error: err,
		Meta:  meta,
	})

	publishError(publisherName, err)
}

func PublishEventResult(publisherName string, topic eventmodels.EventName, event interface{}) {
	publish(publisherName, topic, event)
}

func PublishTerminalError(publisherName string, err error, meta *eventmodels.MetaData) {
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

// APIRequestEvent (creates)-> StreamWriteEvent (creates)-> *StreamWriteProcessedEvent (creates)-> StreamReadEvent (creates)-> **StreamReadProcessedEvent -> APISuccessEvent
// *StreamWriteErrorEvent (creates)-> APIErrorEvent | APIRequestLookup(success)
// **StreamReadErrorEvent (creates)-> APIErrorEvent | APIRequestLookup(success)
func publishWithFlags(publisherName string, topic eventmodels.EventName, event interface{}, logEvent bool) {
	var requestID *string
	switch ev := event.(type) {
	// case eventmodels.APIRequestEvent:
	// case eventmodels.StreamReadEvent:
	case eventmodels.RequestError:
		// RequestErrors should have ResultEvents??
		fmt.Println(ev)
		log.Warnf("deprecated RequestEvent: %v", ev)
	case eventmodels.RequestEvent:
		// _id := ev.GetRequestID().String()
		// requestID = &_id

		// // THIS NEEDS TO GO!!!
		// meta, ok := event.(eventmodels.ResultEvent)
		// if ok {
		// 	meta.GetMetaData().EndProcess(event, nil)
		// }
		log.Warnf("deprecated RequestEvent: %v", ev)
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

func Subscribe(subscriberName string, topic eventmodels.EventName, callbackFn interface{}) {
	if err := bus.SubscribeAsync(string(topic), callbackFn, false); err != nil {
		log.Errorf("[%v] error: %v", subscriberName, err)
	}

	log.Infof("[%v] Subscribed to topic %s", subscriberName, topic)
}
