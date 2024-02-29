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

func (cli *eventStoreDBClient) InsertEvent(ctx context.Context, eventName pubsub.EventName, streamName string, eventType string, data []byte) error {
	eventData := esdb.EventData{
		ContentType: esdb.JsonContentType,
		EventType:   string(eventName),
		Data:        data,
	}

	// todo: verify that the stream is thread safe
	_, err := cli.db.AppendToStream(ctx, streamName, esdb.AppendToStreamOptions{}, eventData)

	return err
}

func (cli *eventStoreDBClient) createAccountRequestHandler(request *eventmodels.CreateAccountRequestEvent) {
	log.Debug("<- eventStoreDBClient.createAccountRequestHandler")

	bytes, err := json.Marshal(request)
	if err != nil {
		pubsub.PublishRequestError("eventStoreDBClient", request, err)
		return
	}

	if err := cli.InsertEvent(context.Background(), pubsub.CreateAccountRequestEvent, "accounts", "CreateAccountRequestEvent", bytes); err != nil {
		pubsub.PublishRequestError("eventStoreDBClient", request, err)
		return
	}
}

func (cli *eventStoreDBClient) readStream(ctx context.Context, stream *esdb.Subscription) {
	for {
		payload := stream.Recv()

		// if err, ok := esdb.FromError(err); !ok {
		// 	if err.Code() == esdb.ErrorCodeResourceNotFound {
		// 		fmt.Print("Stream not found")
		// 	} else if errors.Is(err, io.EOF) {
		// 		break
		// 	} else {
		// 		panic(err)
		// 	}
		// }
		// if errors.Is(err, io.EOF) {
		// 	log.Infof("exiting stream ...")
		// 	return
		// }

		// if err != nil {
		// 	log.Errorf("failed to read event: %v", err)
		// }

		// if event == nil {
		// 	continue
		// }

		if payload.EventAppeared == nil {
			continue
		}

		event := payload.EventAppeared.Event

		switch event.EventType {
		case "CreateAccountRequestEvent":
			var request eventmodels.CreateAccountRequestEvent
			if err := json.Unmarshal(event.Data, &request); err != nil {
				pubsub.PublishError("eventStoreDBClient.CreateAccountRequestEvent", err)
				continue
			}

			pubsub.PublishResult("eventStoreDBClient", pubsub.CreateAccountRequestEventStoredSuccess, &request)
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

	pubsub.Subscribe("eventStoreDBClient", pubsub.CreateAccountRequestEvent, cli.createAccountRequestHandler)

	streamNames := []string{"accounts"}
	for _, streamName := range streamNames {
		subscription, err := cli.db.SubscribeToStream(context.Background(), streamName, esdb.SubscribeToStreamOptions{
			From: esdb.Start{},
		})
		// stream, err := cli.db.ReadStream(ctx, streamName, esdb.ReadStreamOptions{}, 10)
		if err != nil {
			log.Panicf("failed to create stream: %v", err)
		}

		// defer subscription.Close()

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
