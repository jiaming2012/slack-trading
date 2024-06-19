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

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
)

type EsdbEvent[T eventmodels.SavedEvent] struct {
	Event    T
	IsReplay bool
}

type esdbConsumerStream[T eventmodels.SavedEvent] struct {
	wg            *sync.WaitGroup
	db            *esdb.Client
	url           string
	savedEventsCh chan EsdbEvent[T]
	streamName    eventmodels.StreamName
}

func NewESDBConsumerStream[T eventmodels.SavedEvent](wg *sync.WaitGroup, url string, instance T) *esdbConsumerStream[T] {
	return &esdbConsumerStream[T]{
		wg:            wg,
		url:           url,
		savedEventsCh: make(chan EsdbEvent[T]),
		streamName:    instance.GetSavedEventParameters().StreamName,
	}
}

// In order to avoid race conditons and needing to make a copy of saved events on each call, we block the write operation with a mutex until the caller is done reading the data
func (cli *esdbConsumerStream[T]) GetEventCh() <-chan EsdbEvent[T] {
	return cli.savedEventsCh
}

func (cli *esdbConsumerStream[T]) run(ctx context.Context, errCh chan error) {
	cli.wg.Add(1)
	defer cli.wg.Done()

	for {
		select {
		case err := <-errCh:
			log.Panicf("eventStoreDBClient: error channel: %v", err)
		case <-ctx.Done():
			log.Infof("eventStoreDBClient: context done")
			return
		}
	}
}

func (cli *esdbConsumerStream[T]) subscribeToStream(ctx context.Context, streamName eventmodels.StreamName, initialEventNumber uint64) (chan error, error) {
	subscription, err := cli.db.SubscribeToStream(ctx, string(streamName), esdb.SubscribeToStreamOptions{
		From: esdb.Revision(initialEventNumber),
	})

	if err != nil {
		return nil, fmt.Errorf("esdbConsumerStream: failed to subscribe to stream: %v", err)
	}

	log.Infof("esdbConsumerStream: subscribed to stream %s", streamName)

	lastEventNumber := initialEventNumber

	errCh := make(chan error)

	go func() {
		for {
			for {
				event := subscription.Recv()

				if event.SubscriptionDropped != nil {
					log.Infof("esdbConsumerStream: Subscription dropped: %v", event.SubscriptionDropped.Error)
					break
				}

				if event.EventAppeared == nil {
					continue
				}

				if event.CheckPointReached != nil {
					log.Infof("esdbConsumerStream: checkpoint reached: %v\n", event.CheckPointReached)
				}

				ev := event.EventAppeared.Event

				lastEventNumber = event.EventAppeared.OriginalEvent().EventNumber

				if err := cli.processEvent(ctx, ev, false); err != nil {
					errCh <- fmt.Errorf("esdbConsumerStream: failed to process event: %v", err)
					return
				}
			}

			log.Infof("re-subscribing subscription @ pos %v", lastEventNumber)

			subscription, err = cli.db.SubscribeToStream(ctx, string(streamName), esdb.SubscribeToStreamOptions{
				From: esdb.Revision(lastEventNumber),
			})

			if err != nil {
				log.Errorf("esdbConsumerStream: failed to subscribe to stream: %v", err)
			}
		}
	}()

	return errCh, nil
}

func (cli *esdbConsumerStream[T]) processEvent(ctx context.Context, event *esdb.RecordedEvent, isReplay bool) error {
	var savedEvent T

	if err := json.Unmarshal(event.Data, &savedEvent); err != nil {
		return fmt.Errorf("esdbConsumerStream.processEvent: failed to unmarshal event data: %v", err)
	}

	log.Debugf("esdbConsumerStream: processEvent: publishing event %d to savedEventsCh", event.EventNumber)

	select {
	case <-ctx.Done():
		log.Errorf("esdbConsumerStream: processEvent: context done")
		return fmt.Errorf("esdbConsumerStream: processEvent: context done")
	case cli.savedEventsCh <- EsdbEvent[T]{Event: savedEvent, IsReplay: isReplay}:
		log.Debugf("esdbConsumerStream: processEvent: successfully published event %d to savedEventsCh", event.EventNumber)
	}

	return nil
}

func (cli *esdbConsumerStream[T]) replayEvents(ctx context.Context, name eventmodels.StreamName, lastEventNumber uint64) error {
	if lastEventNumber == 0 {
		return nil
	}

	event, err := cli.db.ReadStream(ctx, string(name), esdb.ReadStreamOptions{}, lastEventNumber)
	if err != nil {
		return fmt.Errorf("esdbConsumerStream: failed to read stream %s: %v", name, err)
	}

	for {
		event, err := event.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Println("EOF")
				break
			}

			return fmt.Errorf("esdbConsumerStream: failed to read event from stream: %v", err)
		}

		log.Debugf("esdbConsumerStream: replaying event %d", event.Event.EventNumber)

		if err := cli.processEvent(ctx, event.Event, true); err != nil {
			return fmt.Errorf("esdbConsumerStream: failed to process event: %v", err)
		}
	}

	return nil
}

func (cli *esdbConsumerStream[T]) Start(ctx context.Context) {
	settings, err := esdb.ParseConnectionString(cli.url)
	if err != nil {
		log.Panicf("esdbConsumerStream: failed to parse connection string: %v", err)
	}

	cli.db, err = esdb.NewClient(settings)
	if err != nil {
		log.Panicf("esdbConsumerStream: failed to create client: %v", err)
	}

	log.Debugf("esdbConsumerStream: fetching last event number for stream %s", cli.streamName)

	lastEventNumber, err := eventservices.FindStreamLastEventNumber(ctx, cli.db, cli.streamName)
	if err != nil {
		log.Panicf("esdbConsumerStream: eventStoreDBClient: failed to find last event number: %v", err)
	}

	log.Debugf("esdbConsumerStream: replaying events for stream %s", cli.streamName)

	if err := cli.replayEvents(ctx, cli.streamName, lastEventNumber); err != nil {
		log.Panicf("esdbConsumerStream: eventStoreDBClient: failed to replay events: %v", err)
	}

	log.Debugf("esdbConsumerStream: subscribing to stream %s", cli.streamName)

	var errCh chan error
	if errCh, err = cli.subscribeToStream(ctx, cli.streamName, lastEventNumber); err != nil {
		log.Panicf("eventStoreDBClient: failed to subscribe to stream: %v", err)
	}

	log.Info("esdbConsumerStream: running consumer...")

	go cli.run(ctx, errCh)
}
