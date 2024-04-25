package eventconsumers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/EventStore/EventStore-Client-Go/esdb"

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

func (cli *esdbConsumer) run(ctx context.Context) {
	defer cli.wg.Done()

	timer := time.NewTimer(5 * time.Second)

	for {
		select {
		case <-timer.C:
			cli.mu.Lock()

			fmt.Printf("Found %d option contracts\n", len(cli.optionContracts))

			cli.mu.Unlock()
			timer.Reset(5 * time.Second)
		case <-ctx.Done():
			return
		}
	}
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

		var contract eventmodels.OptionContract
		if err := json.Unmarshal(event.Event.Data, &contract); err != nil {
			return fmt.Errorf("eventStoreDBClient: failed to unmarshal event data: %v", err)
		}

		cli.optionContracts = append(cli.optionContracts, contract)
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

	go cli.run(ctx)
}
