package eventconsumers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventservices"
)

type OptionContractConsumer = esdbConsumer[*eventmodels.OptionContract]

type TrackerConsumer = esdbConsumer[*eventmodels.Tracker]

type esdbConsumer[T eventmodels.SavedEvent] struct {
	wg          *sync.WaitGroup
	db          *esdb.Client
	url         string
	mu          sync.Mutex
	savedEvents []T
	streamName  eventmodels.StreamName
}

func NewESDBConsumer[T eventmodels.SavedEvent](wg *sync.WaitGroup, url string, instance T) *esdbConsumer[T] {
	return &esdbConsumer[T]{
		wg:          wg,
		url:         url,
		savedEvents: make([]T, 0),
		streamName:  instance.GetSavedEventParameters().StreamName,
	}
}

// In order to avoid race conditons and needing to make a copy of saved events on each call, we block the write operation with a mutex until the caller is done reading the data
func (cli *esdbConsumer[T]) GetSavedEvents() ([]T, func()) {
	cli.mu.Lock()

	done := func() {
		cli.mu.Unlock()
	}

	return cli.savedEvents, done
}

func (cli *esdbConsumer[T]) run(ctx context.Context, errCh chan error) {
	defer cli.wg.Done()

	for {
		select {
		case err := <-errCh:
			log.Panicf("eventStoreDBClient: error channel: %v", err)
		case <-ctx.Done():
			return
		}
	}
}

func (cli *esdbConsumer[T]) subscribeToStream(ctx context.Context, streamName eventmodels.StreamName, initialEventNumber uint64) (chan error, error) {
	subscription, err := cli.db.SubscribeToStream(ctx, string(streamName), esdb.SubscribeToStreamOptions{
		From: esdb.Revision(initialEventNumber),
	})

	if err != nil {
		return nil, fmt.Errorf("eventStoreDBClient: failed to subscribe to stream: %v", err)
	}

	log.Infof("subscribed to stream %s", streamName)

	lastEventNumber := initialEventNumber

	errCh := make(chan error)

	go func() {
		for {
			for {
				event := subscription.Recv()

				if event.SubscriptionDropped != nil {
					log.Infof("Subscription dropped: %v", event.SubscriptionDropped.Error)
					break
				}

				if event.EventAppeared == nil {
					continue
				}

				if event.CheckPointReached != nil {
					log.Infof("checkpoint reached: %v\n", event.CheckPointReached)
				}

				ev := event.EventAppeared.Event

				lastEventNumber = event.EventAppeared.OriginalEvent().EventNumber

				if err := cli.processEvent(ev); err != nil {
					errCh <- fmt.Errorf("eventStoreDBClient: failed to process event: %v", err)
					return
				}
			}

			log.Infof("re-subscribing subscription @ pos %v", lastEventNumber)

			subscription, err = cli.db.SubscribeToStream(ctx, string(streamName), esdb.SubscribeToStreamOptions{
				From: esdb.Revision(lastEventNumber),
			})

			if err != nil {
				log.Errorf("eventStoreDBClient: failed to subscribe to stream: %v", err)
			}
		}
	}()

	return errCh, nil
}

func (cli *esdbConsumer[T]) processEvent(event *esdb.RecordedEvent) error {
	var savedEvent T

	if err := json.Unmarshal(event.Data, &savedEvent); err != nil {
		return fmt.Errorf("esdbConsumer.processEvent: failed to unmarshal event data: %v", err)
	}

	cli.mu.Lock()
	cli.savedEvents = append(cli.savedEvents, savedEvent)
	cli.mu.Unlock()

	return nil
}

func (cli *esdbConsumer[T]) replayEvents(ctx context.Context, name eventmodels.StreamName, lastEventNumber uint64) error {
	if lastEventNumber == 0 {
		return nil
	}

	event, err := cli.db.ReadStream(ctx, string(name), esdb.ReadStreamOptions{}, lastEventNumber)
	if err != nil {
		return fmt.Errorf("eventStoreDBClient: failed to read stream %s: %v", name, err)
	}

	for {
		event, err := event.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return fmt.Errorf("eventStoreDBClient: failed to read event from stream: %v", err)
		}

		if err := cli.processEvent(event.Event); err != nil {
			return fmt.Errorf("eventStoreDBClient: failed to process event: %v", err)
		}
	}

	return nil
}

func (cli *esdbConsumer[T]) Start(ctx context.Context) {
	cli.wg.Add(1)

	settings, err := esdb.ParseConnectionString(cli.url)
	if err != nil {
		log.Panicf("failed to parse connection string: %v", err)
	}

	cli.db, err = esdb.NewClient(settings)
	if err != nil {
		log.Panicf("failed to create client: %v", err)
	}

	lastEventNumber, err := eventservices.FindStreamLastEventNumber(cli.db, cli.streamName)
	if err != nil {
		log.Panicf("eventStoreDBClient: failed to find last event number: %v", err)
	}

	if err := cli.replayEvents(ctx, cli.streamName, lastEventNumber); err != nil {
		log.Panicf("eventStoreDBClient: failed to replay events: %v", err)
	}

	var errCh chan error
	if errCh, err = cli.subscribeToStream(ctx, cli.streamName, lastEventNumber); err != nil {
		log.Panicf("eventStoreDBClient: failed to subscribe to stream: %v", err)
	}

	fmt.Println("running consumer...")

	go cli.run(ctx, errCh)
}
