package eventproducers

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/EventStore/EventStore-Client-Go/esdb"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
)

type eventStoreDBClient struct {
	wg              *sync.WaitGroup
	db              *esdb.Client
	mutex           pubsub.SafeMutex
	lastEventNumber uint64
}

func (cli *eventStoreDBClient) insertEvent(ctx context.Context, eventName pubsub.EventName, streamName string, data []byte) error {
	eventData := esdb.EventData{
		ContentType: esdb.JsonContentType,
		EventType:   string(eventName),
		Data:        data,
	}

	// todo: verify that the stream is thread safe
	writeResult, err := cli.db.AppendToStream(ctx, streamName, esdb.AppendToStreamOptions{}, eventData)

	log.Info(writeResult.CommitPosition)

	return err
}

func (cli *eventStoreDBClient) storeRequestEventHandler(request eventmodels.RequestEvent) {
	log.Debug("<- eventStoreDBClient.storeRequestEventHandler")

	bytes, err := json.Marshal(request)
	if err != nil {
		pubsub.PublishRequestError("eventStoreDBClient", request, err)
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
	case *eventmodels.NewSignalRequestEvent:
		if err := cli.insertEvent(context.Background(), pubsub.NewSignalRequestEvent, "accounts", bytes); err != nil {
			pubsub.PublishRequestError("eventStoreDBClient:NewSignalRequest", req, err)
			return
		}
	default:
		pubsub.PublishRequestError("eventStoreDBClient.storeRequestEventHandler", request, fmt.Errorf("unknown request type: %T", request))
		return
	}
}

func (cli *eventStoreDBClient) readStream(stream *esdb.Subscription) {
	for {
		cli.mutex.Lock()

		payload := stream.Recv()

		if payload.SubscriptionDropped != nil {
			// Handle the dropped subscription
			log.Errorf("Subscription dropped: %v", payload.SubscriptionDropped.Error)
			// cli.mutex.Unlock()
			return
		}

		if payload.EventAppeared == nil {
			cli.mutex.Unlock()
			continue
		}

		ev := payload.EventAppeared.Event

		cli.lastEventNumber = payload.EventAppeared.OriginalEvent().EventNumber

		switch pubsub.EventName(ev.EventType) {
		case pubsub.CreateAccountRequestEvent:
			var request eventmodels.CreateAccountRequestEvent
			if err := json.Unmarshal(ev.Data, &request); err != nil {
				pubsub.PublishRequestError("eventStoreDBClient.CreateAccountRequestEvent", &request, err)
				break
			}

			pubsub.PublishResult("eventStoreDBClient", pubsub.CreateAccountRequestEventStoredSuccess, &request)
		case pubsub.CreateAccountStrategyRequestEvent:
			var request eventmodels.CreateAccountStrategyRequestEvent
			if err := json.Unmarshal(ev.Data, &request); err != nil {
				pubsub.PublishRequestError("eventStoreDBClient.CreateAccountStrategyRequestEvent", &request, err)
				break
			}

			pubsub.PublishResult("eventStoreDBClient", pubsub.CreateAccountStrategyRequestEventStoredSuccess, &request)
		case pubsub.NewSignalRequestEvent:
			var request eventmodels.NewSignalRequestEvent
			if err := json.Unmarshal(ev.Data, &request); err != nil {
				pubsub.PublishRequestError("eventStoreDBClient.NewSignalsRequestEvent", &request, err)
				break
			}

			pubsub.PublishResult("eventStoreDBClient", pubsub.NewSignalRequestEventStoredSuccess, &request)
		default:
			// pubsub.PublishError("eventStoreDBClient.readStream", fmt.Errorf("unknown event type: %s", ev.EventType))
			log.Errorf("unknown event type: %s", ev.EventType)
		}
	}
}

func (cli *eventStoreDBClient) wait(event interface{}) {
	log.Debugf("<- eventStoreDBClient.wait: finished processing %v", event)

	switch event.(type) {
	case *eventmodels.CreateAccountRequestEvent:
		cli.mutex.Unlock()
	case *eventmodels.CreateAccountStrategyRequestEvent:
		cli.mutex.Unlock()
	case *eventmodels.NewSignalRequestEvent:
		cli.mutex.Unlock()
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
	pubsub.Subscribe("eventStoreDBClient", pubsub.NewSignalRequestEvent, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", pubsub.ProcessRequestComplete, cli.wait)

	// streamNames := []string{"accounts"}
	// for _, streamName := range streamNames {
	streamName := "accounts"

	subscription, err := cli.db.SubscribeToStream(context.Background(), streamName, esdb.SubscribeToStreamOptions{
		From: esdb.Start{},
	})

	if err != nil {
		log.Panicf("failed to create stream: %v", err)
	}

	go func() {
		for {
			if err == nil {
				cli.readStream(subscription)
			} else {
				log.Errorf("failed to re-create stream: %v", err)
				time.Sleep(5 * time.Second)
			}

			log.Debugf("re-creating subscription @ pos %v", cli.lastEventNumber)

			subscription, err = cli.db.SubscribeToStream(context.Background(), streamName, esdb.SubscribeToStreamOptions{
				From: esdb.Revision(cli.lastEventNumber),
			})
		}
	}()

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
