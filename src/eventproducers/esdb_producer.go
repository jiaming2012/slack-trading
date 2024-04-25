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
	"slack-trading/src/eventservices"
)

type esdbProducer struct {
	wg                     *sync.WaitGroup
	db                     *esdb.Client
	url                    string
	readStreamParams       []eventmodels.StreamParameter
	lastEventNumber        uint64
	allEventsAtStartupRead map[eventmodels.StreamName]chan bool
	startRead              map[eventmodels.StreamName]chan bool
	saga                   map[eventmodels.EventName]pubsub.SagaFlow
}

func (cli *esdbProducer) insertEvent(ctx context.Context, eventName eventmodels.EventName, streamName string, data []byte) error {
	eventData := esdb.EventData{
		ContentType: esdb.JsonContentType,
		EventType:   string(eventName),
		Data:        data,
	}

	// todo: verify that the stream is thread safe
	_, err := cli.db.AppendToStream(ctx, streamName, esdb.AppendToStreamOptions{}, eventData)
	if err != nil {
		return fmt.Errorf("failed to append event to stream: %w", err)
	}

	return nil
}

func (cli *esdbProducer) storeRequestEventHandler(request interface{}) {
	log.Debug("<- eventStoreDBClient.storeRequestEventHandler")

	event, ok := request.(eventmodels.SavedEvent)
	if !ok {
		log.Fatalf("%T does not implement the SavedEvent interface", request)
	}

	bytes, err := json.Marshal(event)
	if err != nil {
		meta := event.GetMetaData()
		pubsub.PublishRequestError("eventStoreDBClient:json.Marshal", err, &meta)
		return
	}

	eventName := event.GetSavedEventParameters().EventName
	streamName := event.GetSavedEventParameters().StreamName

	if err := cli.insertEvent(context.Background(), eventName, string(streamName), bytes); err != nil {
		meta := event.GetMetaData()
		pubsub.PublishRequestError("eventStoreDBClient:cli.insertEvent", err, &meta)
		return
	}

	log.Infof("%s saved to stream %s", eventName, streamName)
}

func (cli *esdbProducer) readStream(streamName eventmodels.StreamName, stream *esdb.Subscription, streamMutex *sync.Mutex, lastEventNumberAtStartup uint64) {
	cli.init()

	if lastEventNumberAtStartup == 0 {
		cli.allEventsAtStartupRead[streamName] <- true
	}

	for {
		payload := stream.Recv()

		if payload.SubscriptionDropped != nil {
			// Handle the dropped subscription
			log.Errorf("Subscription dropped: %v", payload.SubscriptionDropped.Error)
			return
		}

		if payload.EventAppeared == nil {
			streamMutex.Unlock()
			continue
		}

		ev := payload.EventAppeared.Event

		cli.lastEventNumber = payload.EventAppeared.OriginalEvent().EventNumber

		if lastEventNumberAtStartup > 0 && cli.lastEventNumber == lastEventNumberAtStartup {
			ch, found := cli.allEventsAtStartupRead[streamName]
			if !found {
				log.Fatalf("stream %s not found", streamName)
			}

			fmt.Printf("send channel before")
			ch <- true
			fmt.Printf("send channel after")
		}

		// todo: add to interface method
		eventName := eventmodels.EventName(ev.EventType)

		model, found := cli.saga[eventName]
		if !found {
			log.Errorf("unknown event type: %s", ev.EventType)
			continue
		}

		request := model.Generate()
		if err := json.Unmarshal(ev.Data, request); err != nil {
			pubsub.PublishError("eventStoreDBClient.readStream", err)
			continue
		}

		var requestID uuid.UUID
		var isExternalRequest bool
		meta := request.GetMetaData()

		requestID = meta.RequestID
		isExternalRequest = eventmodels.DispatchedRequestExists(requestID)

		request.SetMetaData(&eventmodels.MetaData{
			Mutex:             streamMutex,
			RequestID:         requestID,
			IsExternalRequest: isExternalRequest,
		})

		nextEvent := eventmodels.NewSavedEvent(eventName)

		streamMutex.Lock()
		pubsub.PublishEvent("eventStoreDBClient", nextEvent, request)
	}
}

func (cli *esdbProducer) handleProcessRequestComplete(event interface{}) {
	if req, ok := event.(pubsub.RequestEvent); ok {
		log.Debugf("finished processing request: %s", req.GetMetaData().RequestID.String())

		mutex := req.GetMetaData().Mutex
		if mutex != nil {
			mutex.Unlock()
		}
	}
}

func (cli *esdbProducer) init() {
	cli.saga = pubsub.NewSagaFlow()
}

func (cli *esdbProducer) Start(ctx context.Context) {
	cli.wg.Add(1)

	settings, err := esdb.ParseConnectionString(cli.url)
	if err != nil {
		panic(fmt.Errorf("failed to parse connection string: %w", err))
	}

	cli.db, err = esdb.NewClient(settings)
	if err != nil {
		panic(fmt.Errorf("failed to create client: %w", err))
	}

	pubsub.Subscribe("eventStoreDBClient", eventmodels.CreateNewStockTickEvent, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", eventmodels.CreateNewOptionChainTickEvent, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", eventmodels.CreateAccountRequestEventName, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", eventmodels.CreateAccountStrategyRequestEventName, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", eventmodels.CreateSignalRequestEventName, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", eventmodels.CreateOptionAlertRequestEventName, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", eventmodels.DeleteOptionAlertRequestEventName, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", eventmodels.OptionAlertUpdateEventName, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", eventmodels.CreateOptionContractEvent, cli.storeRequestEventHandler)
	pubsub.Subscribe("eventStoreDBClient", eventmodels.ProcessRequestCompleteEventName, cli.handleProcessRequestComplete)

	for _, param := range cli.readStreamParams {
		mutex := param.Mutex

		lastEventNumber, err := eventservices.FindStreamLastEventNumber(cli.db, param.StreamName)
		if err != nil {
			log.Panicf("eventStoreDBClient: failed to find last event number: %v", err)
		}

		streamName := param.StreamName
		name := string(streamName)
		subscription, err := cli.db.SubscribeToStream(context.Background(), name, esdb.SubscribeToStreamOptions{
			From: esdb.Start{},
		})

		if err != nil {
			log.Panicf("eventStoreDBClient: failed to subscribe to stream: %v", err)
		}

		go func() {
			for {
				if err == nil {
					ch, found := cli.startRead[streamName]
					if !found {
						log.Fatalf("<-cli.startRead: stream %s not found", streamName)
					}

					fmt.Printf("receive channel before")
					<-ch
					fmt.Printf("receive channel after")
					cli.readStream(streamName, subscription, mutex, lastEventNumber)
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

func (cli *esdbProducer) StartRead(name eventmodels.StreamName) {
	channel, found := cli.startRead[name]
	if !found {
		log.Fatalf("stream %s not found", name)
	}

	channel <- true
}

func (cli *esdbProducer) AllEventsAtStartUpRead(streamName eventmodels.StreamName) <-chan bool {
	channel, found := cli.allEventsAtStartupRead[streamName]
	if !found {
		log.Fatalf("esdbProducer:stream %s not found", streamName)
	}

	return channel
}

func NewESDBProducer(wg *sync.WaitGroup, url string, readStreamParams []eventmodels.StreamParameter) *esdbProducer {
	m1 := make(map[eventmodels.StreamName]chan bool)
	m2 := make(map[eventmodels.StreamName]chan bool)

	for _, param := range readStreamParams {
		m1[param.StreamName] = make(chan bool, 1)
		m2[param.StreamName] = make(chan bool)
	}

	return &esdbProducer{
		wg:                     wg,
		url:                    url,
		readStreamParams:       readStreamParams,
		allEventsAtStartupRead: m1,
		startRead:              m2,
	}
}