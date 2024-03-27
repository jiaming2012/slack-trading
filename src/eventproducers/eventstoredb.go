package eventproducers

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/EventStore/EventStore-Client-Go/esdb"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
)

var saga map[eventmodels.EventName]pubsub.SagaFlow

type eventStoreDBClient struct {
	wg                 *sync.WaitGroup
	db                 *esdb.Client
	accountsMutex      sync.Mutex
	optionsAlertsMutex sync.Mutex
	lastEventNumber    uint64
}

func (cli *eventStoreDBClient) insertEvent(ctx context.Context, eventName eventmodels.EventName, streamName string, data []byte) error {
	eventData := esdb.EventData{
		ContentType: esdb.JsonContentType,
		EventType:   string(eventName),
		Data:        data,
	}

	// todo: verify that the stream is thread safe
	writeResult, err := cli.db.AppendToStream(ctx, streamName, esdb.AppendToStreamOptions{}, eventData)
	if err != nil {
		return fmt.Errorf("failed to append event to stream: %w", err)
	}

	log.Info(writeResult.CommitPosition)
	return nil
}

func (cli *eventStoreDBClient) storeRequestEventHandler(event eventmodels.DBInterface) {
	log.Debug("<- eventStoreDBClient.storeRequestEventHandler")

	bytes, err := json.Marshal(event)
	if err != nil {
		// pubsub.PublishRequestError("eventStoreDBClient", request, err)
		return
	}

	// meta := event.GetMetaData()

	eventName := event.GetEventName()
	streamName := event.GetStreamName()

	if err := cli.insertEvent(context.Background(), eventName, streamName, bytes); err != nil {
		// pubsub.PublishRequestError("eventStoreDBClient:CreateAccountRequestEvent", req, err)
		return
	}
}

func (cli *eventStoreDBClient) readStream(stream *esdb.Subscription, streamName string, streamMutex *sync.Mutex) {
	cli.init()

	for {
		payload := stream.Recv()

		if payload.SubscriptionDropped != nil {
			// Handle the dropped subscription
			log.Errorf("Subscription dropped: %v", payload.SubscriptionDropped.Error)
			return
		}

		if payload.EventAppeared == nil {
			if streamName == "accounts" {
				cli.accountsMutex.Unlock()
			} else if streamName == "option-alerts" {
				cli.optionsAlertsMutex.Unlock()
			}

			continue
		}

		ev := payload.EventAppeared.Event

		cli.lastEventNumber = payload.EventAppeared.OriginalEvent().EventNumber

		model, found := saga[eventmodels.EventName(ev.EventType)]
		if !found {
			log.Errorf("unknown event type: %s", ev.EventType)
			continue
		}

		request := model.Generator()
		if err := json.Unmarshal(ev.Data, request); err != nil {
			pubsub.PublishEventError("eventStoreDBClient.readStream", err)
			continue
		}

		meta := request.GetMetaData()
		var requestID uuid.UUID
		if meta == nil {
			requestID = uuid.Nil
		} else {
			requestID = meta.RequestID
		}

		request.SetMetaData(&eventmodels.MetaData{
			Mutex:     streamMutex,
			RequestID: requestID,
		})

		streamMutex.Lock()
		pubsub.PublishEventResult("eventStoreDBClient", model.NextEvent, request)
	}
}

func (cli *eventStoreDBClient) handleProcessRequestComplete(event interface{}) {
	log.Debugf("<- eventStoreDBClient.handleProcessRequestComplete: finished processing %v", event)

	if req, ok := event.(pubsub.TerminalRequest); ok {
		mutex := req.GetMetaData().Mutex
		if mutex != nil {
			mutex.Unlock()
		}
	}
}

func (cli *eventStoreDBClient) init() {
	saga = map[eventmodels.EventName]pubsub.SagaFlow{
		// pubsub.CreateAccountRequestEvent: {
		// 	Generator: func() interface{} { return &eventmodels.CreateAccountRequestEvent{} },
		// 	NextEvent: pubsub.CreateAccountRequestSavedEventName,
		// 	Lock:      &cli.accountsMutex,
		// },
		// pubsub.CreateAccountStrategyRequestEvent: {
		// 	Generator: func() interface{} { return &eventmodels.CreateAccountStrategyRequestEvent{} },
		// 	NextEvent: pubsub.CreateAccountStrategyRequestSavedEventName,
		// 	Lock:      &cli.accountsMutex,
		// },
		// pubsub.CreateSignalRequestEvent: {
		// 	Generator: func() interface{} { return &eventmodels.CreateSignalRequest{} },
		// 	NextEvent: pubsub.CreateSignalRequestSavedEventName,
		// 	Lock:      &cli.accountsMutex,
		// },
		eventmodels.CreateOptionAlertRequestEventName: {
			Generator: func() pubsub.TerminalRequest { return &eventmodels.CreateOptionAlertRequestEvent{} },
			NextEvent: eventmodels.CreateOptionAlertRequestSavedEventName,
		},
		eventmodels.DeleteOptionAlertRequestEventName: {
			Generator: func() pubsub.TerminalRequest { return &eventmodels.DeleteOptionAlertRequestEvent{} },
			NextEvent: eventmodels.DeleteOptionAlertRequestSavedEventName,
		},
		eventmodels.OptionAlertUpdateEventName: {
			Generator: func() pubsub.TerminalRequest { return &eventmodels.OptionAlertUpdateEvent{} },
			NextEvent: eventmodels.OptionAlertUpdateSavedEventName,
		},
	}
}

func (cli *eventStoreDBClient) Start(ctx context.Context, url string) {
	cli.wg.Add(1)

	settings, err := esdb.ParseConnectionString(url)
	if err != nil {
		panic(fmt.Errorf("failed to parse connection string: %w", err))
	}

	cli.db, err = esdb.NewClient(settings)
	if err != nil {
		panic(fmt.Errorf("failed to create client: %w", err))
	}

	pubsub.Subscribe("eventStoreDBClient", eventmodels.CreateAccountRequestEventName, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", eventmodels.CreateAccountStrategyRequestEventName, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", eventmodels.CreateSignalRequestEventName, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", eventmodels.CreateOptionAlertRequestEventName, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", eventmodels.DeleteOptionAlertRequestEventName, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", eventmodels.OptionAlertUpdateEventName, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", eventmodels.ProcessRequestCompleteEventName, cli.handleProcessRequestComplete)

	// streamNames := []string{"accounts", "option-alerts"}
	streamNames := []string{"option-alerts"}
	for _, streamName := range streamNames {
		name := streamName

		subscription, err := cli.db.SubscribeToStream(context.Background(), name, esdb.SubscribeToStreamOptions{
			From: esdb.Start{},
		})

		if err != nil {
			log.Panicf("failed to subscribe to stream: %v", err)
		}

		go func() {
			for {
				if err == nil {
					cli.readStream(subscription, name, &cli.optionsAlertsMutex)
				} else {
					log.Errorf("failed to re-subscribe stream: %v", err)
					time.Sleep(5 * time.Second)
				}

				log.Debugf("re-subscribing subscription @ pos %v", cli.lastEventNumber)

				subscription, err = cli.db.SubscribeToStream(context.Background(), name, esdb.SubscribeToStreamOptions{
					From: esdb.Revision(cli.lastEventNumber),
				})
			}
		}()
	}

	go func() {
		defer cli.wg.Done()
		defer cli.db.Close()

		for range ctx.Done() {
			fmt.Printf("\nstopping EventStoreDB producer\n")
			return
		}
	}()
}

func NewEventStoreDBClient(wg *sync.WaitGroup) *eventStoreDBClient {
	return &eventStoreDBClient{
		wg: wg,
	}
}
