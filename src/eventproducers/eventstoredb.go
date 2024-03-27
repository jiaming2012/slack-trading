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

var saga map[pubsub.EventName]pubsub.SagaFlow

type eventStoreDBClient struct {
	wg                 *sync.WaitGroup
	db                 *esdb.Client
	accountsMutex      sync.Mutex
	optionsAlertsMutex sync.Mutex
	lastEventNumber    uint64
}

func (cli *eventStoreDBClient) insertEvent(ctx context.Context, eventName pubsub.EventName, streamName string, data []byte) error {
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

func (cli *eventStoreDBClient) storeRequestEventHandler(request interface{}) {
	log.Debug("<- eventStoreDBClient.storeRequestEventHandler")

	bytes, err := json.Marshal(request)
	if err != nil {
		// pubsub.PublishRequestError("eventStoreDBClient", request, err)
		return
	}

	switch req := request.(type) {
	case *eventmodels.CreateAccountRequestEvent:
		if err := cli.insertEvent(context.Background(), pubsub.CreateAccountRequestEvent, "accounts", bytes); err != nil {
			pubsub.PublishRequestError("eventStoreDBClient:CreateAccountRequestEvent", req, err)
			return
		}
	case *eventmodels.CreateAccountStrategyRequestEvent:
		if err := cli.insertEvent(context.Background(), pubsub.CreateAccountStrategyRequestEvent, "accounts", bytes); err != nil {
			pubsub.PublishRequestError("eventStoreDBClient:CreateAccountStrategyRequestEvent", req, err)
			return
		}
	case *eventmodels.CreateSignalRequest:
		if err := cli.insertEvent(context.Background(), pubsub.CreateSignalRequestEvent, "accounts", bytes); err != nil {
			pubsub.PublishRequestError("eventStoreDBClient:NewSignalRequest", req, err)
			return
		}
	case *eventmodels.CreateOptionAlertRequestEvent:
		if err := cli.insertEvent(context.Background(), pubsub.CreateOptionAlertRequestEvent, "option-alerts", bytes); err != nil {
			// pubsub.PublishRequestError("eventStoreDBClient:CreateOptionAlertRequestEvent", req, err)
			return
		}
	case *eventmodels.DeleteOptionAlertRequestEvent:
		if err := cli.insertEvent(context.Background(), pubsub.DeleteOptionAlertRequestEvent, "option-alerts", bytes); err != nil {
			// pubsub.PublishRequestError("eventStoreDBClient:DeleteOptionAlertRequestEvent", req, err)
			return
		}
	case *eventmodels.OptionAlertUpdateEvent:
		if err := cli.insertEvent(context.Background(), pubsub.OptionAlertUpdateEvent, "option-alerts", bytes); err != nil {
			// pubsub.PublishRequestError("eventStoreDBClient:OptionAlertUpdateEvent", req, err)
			return
		}
	default:
		// pubsub.PublishRequestError("eventStoreDBClient.storeRequestEventHandler", request, fmt.Errorf("unknown request type: %T", request))
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

		model, found := saga[pubsub.EventName(ev.EventType)]
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

	// switch event.(type) {
	// case *eventmodels.CreateAccountResponseEvent:
	// 	cli.accountsMutex.Unlock()
	// case *eventmodels.CreateAccountStrategyResponseEvent:
	// 	cli.accountsMutex.Unlock()
	// case *eventmodels.CreateSignalResponseEvent:
	// 	cli.accountsMutex.Unlock()
	// case *eventmodels.CreateOptionAlertResponseEvent:
	// 	cli.optionsAlertsMutex.Unlock()
	// case *eventmodels.DeleteOptionAlertResponseEvent:
	// 	cli.optionsAlertsMutex.Unlock()
	// // too many places to add
	// case *eventmodels.OptionAlertUpdateCompletedEvent:
	// 	cli.optionsAlertsMutex.Unlock()
	// }
}

func (cli *eventStoreDBClient) init() {
	saga = map[pubsub.EventName]pubsub.SagaFlow{
		// pubsub.CreateAccountRequestEvent: {
		// 	Generator: func() interface{} { return &eventmodels.CreateAccountRequestEvent{} },
		// 	NextEvent: pubsub.CreateAccountRequestEventStoredSuccess,
		// 	Lock:      &cli.accountsMutex,
		// },
		// pubsub.CreateAccountStrategyRequestEvent: {
		// 	Generator: func() interface{} { return &eventmodels.CreateAccountStrategyRequestEvent{} },
		// 	NextEvent: pubsub.CreateAccountStrategyRequestEventStoredSuccess,
		// 	Lock:      &cli.accountsMutex,
		// },
		// pubsub.CreateSignalRequestEvent: {
		// 	Generator: func() interface{} { return &eventmodels.CreateSignalRequest{} },
		// 	NextEvent: pubsub.CreateSignalRequestStoredSuccessEvent,
		// 	Lock:      &cli.accountsMutex,
		// },
		pubsub.CreateOptionAlertRequestEvent: {
			Generator: func() pubsub.TerminalRequest { return &eventmodels.CreateOptionAlertRequestEvent{} },
			NextEvent: pubsub.CreateOptionAlertRequestEventStoredSuccess,
		},
		pubsub.DeleteOptionAlertRequestEvent: {
			Generator: func() pubsub.TerminalRequest { return &eventmodels.DeleteOptionAlertRequestEvent{} },
			NextEvent: pubsub.DeleteOptionAlertRequestEventStoredSuccess,
		},
		pubsub.OptionAlertUpdateEvent: {
			Generator: func() pubsub.TerminalRequest { return &eventmodels.OptionAlertUpdateEvent{} },
			NextEvent: pubsub.OptionAlertUpdateSavedEvent,
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

	pubsub.Subscribe("eventStoreDBClient", pubsub.CreateAccountRequestEvent, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", pubsub.CreateAccountStrategyRequestEvent, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", pubsub.CreateSignalRequestEvent, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", pubsub.CreateOptionAlertRequestEvent, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", pubsub.DeleteOptionAlertRequestEvent, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", pubsub.OptionAlertUpdateEvent, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", pubsub.ProcessRequestComplete, cli.handleProcessRequestComplete)

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
