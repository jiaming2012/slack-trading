package eventproducers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	pubsub "github.com/jiaming2012/slack-trading/src/eventpubsub"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

type EsdbProducer struct {
	wg                     *sync.WaitGroup
	db                     *esdb.Client
	url                    string
	readStreamParams       []eventmodels.StreamParameter
	lastEventNumber        uint64
	allEventsAtStartupRead map[eventmodels.StreamName]chan bool
	startRead              map[eventmodels.StreamName]chan bool
	saga                   map[eventmodels.EventName]pubsub.SagaFlow
}

func (cli *EsdbProducer) insertEvent(ctx context.Context, eventName eventmodels.EventName, streamName string, meta []byte, data []byte) error {
	eventData := esdb.EventData{
		ContentType: esdb.ContentTypeJson,
		EventType:   string(eventName),
		Data:        data,
	}

	if meta != nil {
		eventData.Metadata = meta
	}

	if cli.db == nil {
		return errors.New("db is nil")
	}

	// todo: verify that the stream is thread safe
	_, err := cli.db.AppendToStream(ctx, streamName, esdb.AppendToStreamOptions{}, eventData)
	if err != nil {
		return fmt.Errorf("failed to append event to stream: %w", err)
	}

	return nil
}

func (cli *EsdbProducer) insertData(ctx context.Context, event eventmodels.SavedEvent, data map[string]interface{}) error {
	// set the event streamID
	eventID := eventmodels.EventStreamID(uuid.New())
	metaData := event.GetMetaData()
	metaData.SetEventStreamID(eventID)

	// set the schema version
	schemaVersion := event.GetSavedEventParameters().SchemaVersion
	metaData.SetSchemaVersion(schemaVersion)

	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	params := event.GetSavedEventParameters()

	eventName := params.EventName
	streamName := params.StreamName

	log.Debugf("%s saving to stream %s ...", eventName, streamName)

	if err := cli.insertEvent(ctx, eventName, string(streamName), nil, bytes); err != nil {
		return fmt.Errorf("EsdbProducer: failed to insert event: %w", err)
	}

	return nil
}

func (cli *EsdbProducer) insert(ctx context.Context, event eventmodels.SavedEvent) error {
	// todo: unify metadata with the structs metadata field
	// set the metadata
	var metaBytes []byte

	span := trace.SpanFromContext(ctx)

	if span.SpanContext().IsValid() {
		serializedSpanCtx, err := utils.SerializeTraceContext(span.SpanContext())
		if err != nil {
			return fmt.Errorf("failed to serialize trace context: %w", err)
		}

		meta := eventmodels.EsdbMetadata{
			SpanContext: serializedSpanCtx,
		}

		bytes, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		metaBytes = bytes
	}

	// set the event streamID
	eventID := eventmodels.EventStreamID(uuid.New())
	metaData := event.GetMetaData()
	metaData.SetEventStreamID(eventID)

	// set the schema version
	schemaVersion := event.GetSavedEventParameters().SchemaVersion
	metaData.SetSchemaVersion(schemaVersion)

	bytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	params := event.GetSavedEventParameters()

	eventName := params.EventName
	streamName := params.StreamName

	log.Debugf("%s saving to stream %s ...", eventName, streamName)

	if err := cli.insertEvent(ctx, eventName, string(streamName), metaBytes, bytes); err != nil {
		return fmt.Errorf("EsdbProducer: failed to insert event: %w", err)
	}

	return nil
}

func (cli *EsdbProducer) ProcessSaveCreateSignalRequestEvent(ctx context.Context, request *eventmodels.CreateSignalRequestEventV1DTO) (bool, error) {
	tracer := otel.Tracer("ProcessSaveCreateSignalRequestEvent")
	ctx, span := tracer.Start(ctx, "ProcessSaveCreateSignalRequestEvent")
	defer span.End()

	logger := log.WithContext(ctx)

	logger.WithFields(log.Fields{
		"event": "signal",
	}).Infof("<- esdbProducer.handleSaveCreateSignalRequestEvent, request: %v", request)

	if isValid, err := request.ValidateV2(ctx); isValid {
		if err != nil {
			return true, fmt.Errorf("ProcessSaveCreateSignalRequestEvent: valid signal, but avoid processing due to error: %v", err)
		}

		logger.Infof("saving signal %s", request.Name)

		// save the signal
		if err := cli.insert(ctx, request); err != nil {
			return false, fmt.Errorf("failed to save signal: %w", err)
		}

		span.AddEvent("Signal saved")

		now := time.Now().UTC()

		tracker, err := request.ConvertToTracker(now)
		if err != nil {
			return false, fmt.Errorf("failed to convert signal to tracker: %w", err)
		}

		if err := cli.insert(ctx, tracker); err != nil {
			return false, fmt.Errorf("failed to save tracker: %w", err)
		}

		span.AddEvent("Tracker saved")
	} else {
		return true, fmt.Errorf("ProcessSaveCreateSignalRequestEvent: invalid signal: %v", err)
	}

	return true, nil
}

func (cli *EsdbProducer) handleSaveCreateSignalRequestEvent(request *eventmodels.CreateSignalRequestEventV1DTO) {
	if ok, err := cli.ProcessSaveCreateSignalRequestEvent(context.Background(), request); err != nil {
		if ok {
			log.WithFields(log.Fields{
				"event": "signal",
			}).Debugf("handleSaveCreateSignalRequestEvent: %v", err)
		} else {
			meta := request.GetMetaData()
			pubsub.PublishRequestError("esdbProducer:cli.handleSaveCreateSignalRequestEvent", err, meta)
			return
		}
	}

	pubsub.PublishCompletedResponse("esdbProducer:cli.handleSaveCreateSignalRequest", &eventmodels.CreateSignalResponseEvent{
		Name: request.Name,
	}, request.GetMetaData())
}

func (cli *EsdbProducer) saveRequest(ctx context.Context, request interface{}) error {
	event, ok := request.(eventmodels.SavedEvent)
	if !ok {
		log.Fatalf("%T does not implement the SavedEvent interface", request)
	}

	if err := cli.saveEvent(ctx, event); err != nil {
		return fmt.Errorf("EsdbProducer: failed to save event: %w", err)
	}

	return nil
}

func (cli *EsdbProducer) handleSaveRequest(request interface{}) {
	if err := cli.saveRequest(context.Background(), request); err != nil {
		meta := request.(eventmodels.SavedEvent).GetMetaData()
		pubsub.PublishRequestError("esdbProducer:cli.saveEvent", err, meta)
	}
}

func (cli *EsdbProducer) saveEvent(ctx context.Context, event eventmodels.SavedEvent) error {
	log.WithField("event", event.GetSavedEventParameters().EventName).Debug("EsdbProducer.saveEvent")

	if err := cli.insert(ctx, event); err != nil {
		return fmt.Errorf("EsdbProducer: failed to insert event: %w", err)
	}

	return nil
}

// todo: replace in favor of esdbConsumer
func (cli *EsdbProducer) readStreamDeprecated(streamName eventmodels.StreamName, stream *esdb.Subscription, streamMutex *sync.Mutex, lastEventNumberAtStartup uint64) {
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

			ch <- true
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
			pubsub.PublishError("esdbProducer.readStream", err)
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
		pubsub.PublishEvent("esdbProducer", nextEvent, request)
	}
}

func (cli *EsdbProducer) handleProcessRequestComplete(event interface{}) {
	if req, ok := event.(pubsub.RequestEvent); ok {
		log.Debugf("finished processing request: %s", req.GetMetaData().RequestID.String())

		mutex := req.GetMetaData().Mutex
		if mutex != nil {
			mutex.Unlock()
		}
	}
}

func (cli *EsdbProducer) init() {
	cli.saga = pubsub.NewSagaFlow()
}

func (cli *EsdbProducer) Start(ctx context.Context, fxTicksCh <-chan *eventmodels.FxTick) {
	cli.wg.Add(1)

	settings, err := esdb.ParseConnectionString(cli.url)
	if err != nil {
		panic(fmt.Errorf("failed to parse connection string: %w", err))
	}

	cli.db, err = esdb.NewClient(settings)
	if err != nil {
		panic(fmt.Errorf("failed to create client: %w", err))
	}

	cli.wg.Add(1)

	go func() {
		defer cli.wg.Done()

		for {
			select {
			case fxTick := <-fxTicksCh:
				if err := cli.saveRequest(ctx, fxTick); err != nil {
					pubsub.PublishError("esdbProducer: failed to save candle: ", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	pubsub.Subscribe("esdbProducer", eventmodels.CreateNewStockTickEvent, cli.handleSaveRequest)
	pubsub.Subscribe("esdbProducer", eventmodels.CreateNewOptionChainTickEvent, cli.handleSaveRequest)
	pubsub.Subscribe("esdbProducer", eventmodels.CreateAccountRequestEventName, cli.handleSaveRequest)
	pubsub.Subscribe("esdbProducer", eventmodels.CreateAccountStrategyRequestEventName, cli.handleSaveRequest)
	pubsub.Subscribe("esdbProducer", eventmodels.CreateSignalRequestEventName, cli.handleSaveCreateSignalRequestEvent)
	pubsub.Subscribe("esdbProducer", eventmodels.CreateOptionAlertRequestEventName, cli.handleSaveRequest)
	pubsub.Subscribe("esdbProducer", eventmodels.DeleteOptionAlertRequestEventName, cli.handleSaveRequest)
	pubsub.Subscribe("esdbProducer", eventmodels.OptionAlertUpdateEventName, cli.handleSaveRequest)
	pubsub.Subscribe("esdbProducer", eventmodels.CreateOptionContractEvent, cli.handleSaveRequest)
	pubsub.Subscribe("esdbProducer", eventmodels.ProcessRequestCompleteEventName, cli.handleProcessRequestComplete)

	for _, param := range cli.readStreamParams {
		mutex := param.Mutex

		lastEventNumber, err := eventservices.FindStreamLastEventNumber(ctx, cli.db, param.StreamName)
		if err != nil {
			log.Panicf("esdbProducer: failed to find last event number: %v", err)
		}

		streamName := param.StreamName
		name := string(streamName)
		subscription, err := cli.db.SubscribeToStream(context.Background(), name, esdb.SubscribeToStreamOptions{
			From: esdb.Start{},
		})

		if err != nil {
			log.Panicf("esdbProducer: failed to subscribe to stream: %v", err)
		}

		go func() {
			for {
				if err == nil {
					ch, found := cli.startRead[streamName]
					if !found {
						log.Fatalf("<-cli.startRead: stream %s not found", streamName)
					}

					<-ch
					cli.readStreamDeprecated(streamName, subscription, mutex, lastEventNumber)
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

func (cli *EsdbProducer) SaveData(ctx context.Context, event eventmodels.SavedEvent, data map[string]interface{}) error {
	return cli.insertData(ctx, event, data)
}

func (cli *EsdbProducer) Save(ctx context.Context, event eventmodels.SavedEvent) error {
	return cli.insert(ctx, event)
}

func (cli *EsdbProducer) StartRead(name eventmodels.StreamName) {
	channel, found := cli.startRead[name]
	if !found {
		log.Fatalf("stream %s not found", name)
	}

	channel <- true
}

func (cli *EsdbProducer) AllEventsAtStartUpRead(streamName eventmodels.StreamName) <-chan bool {
	channel, found := cli.allEventsAtStartupRead[streamName]
	if !found {
		log.Fatalf("esdbProducer:stream %s not found", streamName)
	}

	return channel
}

func (cli *EsdbProducer) GetClient() *esdb.Client {
	return cli.db
}

func NewESDBProducer(wg *sync.WaitGroup, url string, readStreamParams []eventmodels.StreamParameter) *EsdbProducer {
	m1 := make(map[eventmodels.StreamName]chan bool)
	m2 := make(map[eventmodels.StreamName]chan bool)

	for _, param := range readStreamParams {
		m1[param.StreamName] = make(chan bool, 1)
		m2[param.StreamName] = make(chan bool)
	}

	return &EsdbProducer{
		wg:                     wg,
		url:                    url,
		readStreamParams:       readStreamParams,
		allEventsAtStartupRead: m1,
		startRead:              m2,
	}
}
