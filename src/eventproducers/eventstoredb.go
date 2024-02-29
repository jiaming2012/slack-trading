package eventproducers

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/EventStore/EventStore-Client-Go/esdb"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
)

type eventStoreDBClient struct {
	wg *sync.WaitGroup
	db *esdb.Client
}

func (cli *eventStoreDBClient) insertEvent(ctx context.Context, eventName pubsub.EventName, streamName string, data []byte) error {
	eventData := esdb.EventData{
		ContentType: esdb.JsonContentType,
		EventType:   string(eventName),
		Data:        data,
	}

	// todo: verify that the stream is thread safe
	_, err := cli.db.AppendToStream(ctx, streamName, esdb.AppendToStreamOptions{}, eventData)

	return err
}

func (cli *eventStoreDBClient) storeRequestEventHandler(request interface{}) {
	log.Debug("<- eventStoreDBClient.storeRequestEventHandler")

	bytes, err := json.Marshal(request)
	if err != nil {
		pubsub.PublishError("eventStoreDBClient", err)
		return
	}

	switch req := request.(type) {
	case *eventmodels.CreateAccountRequestEvent:
		if err := cli.insertEvent(context.Background(), pubsub.CreateAccountRequestEvent, "accounts", bytes); err != nil {
			pubsub.PublishRequestError("eventStoreDBClient", req, err)
			return
		}
	case *eventmodels.CreateAccountStrategyRequestEvent:
		if err := cli.insertEvent(context.Background(), pubsub.CreateAccountStrategyRequestEvent, "accounts", bytes); err != nil {
			pubsub.PublishRequestError("eventStoreDBClient", req, err)
			return
		}
	default:
		pubsub.PublishError("eventStoreDBClient.storeRequestEventHandler", fmt.Errorf("unknown request type: %T", request))
		return
	}
}

func (cli *eventStoreDBClient) readStream(ctx context.Context, stream *esdb.Subscription) {
	for {
		payload := stream.Recv()

		if payload.EventAppeared == nil {
			continue
		}

		ev := payload.EventAppeared.Event

		switch pubsub.EventName(ev.EventType) {
		case pubsub.CreateAccountRequestEvent:
			var request eventmodels.CreateAccountRequestEvent
			if err := json.Unmarshal(ev.Data, &request); err != nil {
				pubsub.PublishError("eventStoreDBClient.CreateAccountRequestEvent", err)
				continue
			}

			pubsub.PublishResult("eventStoreDBClient", pubsub.CreateAccountRequestEventStoredSuccess, &request)
		case pubsub.CreateAccountStrategyRequestEvent:
			var request eventmodels.CreateAccountStrategyRequestEvent
			if err := json.Unmarshal(ev.Data, &request); err != nil {
				pubsub.PublishError("eventStoreDBClient.CreateAccountStrategyRequestEvent", err)
				continue
			}

			pubsub.PublishResult("eventStoreDBClient", pubsub.CreateAccountStrategyRequestEventStoredSuccess, &request)
		default:
			pubsub.PublishError("eventStoreDBClient.readStream", fmt.Errorf("unknown event type: %s", ev.EventType))
		}
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

	streamNames := []string{"accounts"}
	for _, streamName := range streamNames {
		subscription, err := cli.db.SubscribeToStream(context.Background(), streamName, esdb.SubscribeToStreamOptions{
			From: esdb.Start{},
		})

		if err != nil {
			log.Panicf("failed to create stream: %v", err)
		}

		go cli.readStream(ctx, subscription)
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
