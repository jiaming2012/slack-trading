package eventconsumers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventservices"
)

type esdbConsumer struct {
	wg              *sync.WaitGroup
	db              *esdb.Client
	url             string
	mu              sync.Mutex
	optionContracts []eventmodels.OptionContract
	done            chan bool
}

func NewESDBConsumer(wg *sync.WaitGroup, url string) *esdbConsumer {
	return &esdbConsumer{
		wg:   wg,
		url:  url,
		done: make(chan bool),
	}
}

// In order to avoid race conditons and copying optionContracts, we block the write operation with a mutex until the caller is done reading the data
func (cli *esdbConsumer) GetOptionContracts() ([]eventmodels.OptionContract, <-chan bool) {
	cli.mu.Lock()
	return cli.optionContracts, cli.done
}

func (cli *esdbConsumer) run(ctx context.Context, errCh chan error) {
	defer cli.wg.Done()

	timer := time.NewTimer(5 * time.Second)

	for {
		select {
		case <-timer.C:
			cli.mu.Lock()

			fmt.Printf("Found %d option contracts\n", len(cli.optionContracts))

			cli.mu.Unlock()
			timer.Reset(5 * time.Second)
		case err := <-errCh:
			log.Panicf("eventStoreDBClient: error channel: %v", err)
		case <-ctx.Done():
			return
		}
	}
}

func (cli *esdbConsumer) subscribeToStream(ctx context.Context, streamName eventmodels.StreamName, initialEventNumber uint64) (error, chan error) {
	subscription, err := cli.db.SubscribeToStream(ctx, string(streamName), esdb.SubscribeToStreamOptions{
		From: esdb.Revision(initialEventNumber),
	})

	if err != nil {
		return fmt.Errorf("eventStoreDBClient: failed to subscribe to stream: %v", err), nil
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

	return nil, errCh
}

func (cli *esdbConsumer) processEvent(event *esdb.RecordedEvent) error {
	var contract eventmodels.OptionContract
	if err := json.Unmarshal(event.Data, &contract); err != nil {
		return fmt.Errorf("esdbConsumer.processEvent: failed to unmarshal event data: %v", err)
	}

	cli.optionContracts = append(cli.optionContracts, contract)
	return nil
}

func (cli *esdbConsumer) replayEvents(ctx context.Context, name eventmodels.StreamName, lastEventNumber uint64) error {
	cli.mu.Lock()
	defer cli.mu.Unlock()

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

func (cli *esdbConsumer) Start(ctx context.Context, name eventmodels.StreamName) {
	cli.wg.Add(1)

	settings, err := esdb.ParseConnectionString(cli.url)
	if err != nil {
		panic(fmt.Errorf("failed to parse connection string: %w", err))
	}

	cli.db, err = esdb.NewClient(settings)
	if err != nil {
		panic(fmt.Errorf("failed to create client: %w", err))
	}

	lastEventNumber, err := eventservices.FindStreamLastEventNumber(cli.db, name)
	if err != nil {
		log.Panicf("eventStoreDBClient: failed to find last event number: %v", err)
	}

	if err := cli.replayEvents(ctx, name, lastEventNumber); err != nil {
		log.Panicf("eventStoreDBClient: failed to replay events: %v", err)
	}

	var errCh chan error
	if err, errCh = cli.subscribeToStream(ctx, name, lastEventNumber); err != nil {
		log.Panicf("eventStoreDBClient: failed to subscribe to stream: %v", err)
	}

	fmt.Println("running consumer...")

	go cli.run(ctx, errCh)
}
