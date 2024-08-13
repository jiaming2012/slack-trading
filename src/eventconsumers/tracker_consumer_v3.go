package eventconsumers

import (
	"context"
	"fmt"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdk_trace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type TrackerConsumerV3 struct {
	client          *TrackerV3Client
	state           map[string]string
	signalTriggered chan eventmodels.SignalTriggeredEvent
	mutex           sync.Mutex
}

func (t *TrackerConsumerV3) CloseSignalTriggeredCh() {
	close(t.signalTriggered)
}

func (t *TrackerConsumerV3) GetState() (state map[string]string, unlock func()) {
	t.mutex.Lock()
	state = t.state
	return state, t.mutex.Unlock
}

func (t *TrackerConsumerV3) GetSignalTriggeredCh() <-chan eventmodels.SignalTriggeredEvent {
	return t.signalTriggered
}

func (t *TrackerConsumerV3) checkSupertrendH1StochRsiDown(ctx context.Context, symbol eventmodels.StockSymbol) bool {
	ctx, span := otel.Tracer("tracker_v3_consumer").Start(ctx, "checkSupertrendH1StochRsiDown")
	defer span.End()

	logger := log.WithContext(ctx)

	state, unlock := t.GetState()
	defer unlock()

	m15SignalKey := fmt.Sprintf("%s-15-stochastic_rsi", symbol)
	m15Signal, found := state[m15SignalKey]
	if !found {
		logger.WithField("event", "signal").Warnf("checkSupertrendH1StochRsiDown: Signal not found: %s. Expected if signal was never received.", m15SignalKey)
		return false
	}

	h1SignalKey := fmt.Sprintf("%s-60-supertrend", symbol)
	h1Signal, found := state[h1SignalKey]
	if !found {
		logger.WithField("event", "signal").Warnf("checkSupertrendH1StochRsiDown: Signal not found: %s. Expected if signal was never received.", h1SignalKey)
		return false
	}

	if m15Signal == "sell" && h1Signal == "sell" {
		logger.WithField("event", "signal").Infof("checkSupertrendH1StochRsiDown: triggered for %v", symbol)
		return true
	}

	logger.WithField("event", "signal").Debugf("checkSupertrendH1StochRsiDown NOT triggered for %v, m15Signal=%v, h1Signal=%v", symbol, m15Signal, h1Signal)
	return false
}

func (t *TrackerConsumerV3) checkSupertrendH4H1StochRsiDown(ctx context.Context, symbol eventmodels.StockSymbol) bool {
	ctx, span := otel.Tracer("tracker_v3_consumer").Start(ctx, "checkSupertrendH4H1StochRsiDown")
	defer span.End()

	logger := log.WithContext(ctx)

	state, unlock := t.GetState()
	defer unlock()

	m15SignalKey := fmt.Sprintf("%s-15-stochastic_rsi", symbol)
	m15Signal, found := state[m15SignalKey]
	if !found {
		logger.WithField("event", "signal").Warnf("checkSupertrendH4H1StochRsiDown: Signal not found: %s. Expected if signal was never received.", m15SignalKey)
		return false
	}

	h1SignalKey := fmt.Sprintf("%s-60-supertrend", symbol)
	h1Signal, found := state[h1SignalKey]
	if !found {
		logger.WithField("event", "signal").Warnf("checkSupertrendH4H1StochRsiDown: Signal not found: %s. Expected if signal was never received.", h1SignalKey)
		return false
	}

	h4SignalKey := fmt.Sprintf("%s-240-supertrend", symbol)
	h4Signal, found := state[h4SignalKey]
	if !found {
		logger.WithField("event", "signal").Warnf("checkSupertrendH4H1StochRsiDown: Signal not found: %s. Expected if signal was never received.", h4SignalKey)
		return false
	}

	if m15Signal == "sell" && h1Signal == "sell" && h4Signal == "sell" {
		logger.WithField("event", "signal").Infof("checkSupertrendH4H1StochRsiDown: triggered for %v", symbol)
		return true
	}

	logger.WithField("event", "signal").Debugf("checkSupertrendH4H1StochRsiDown NOT triggered for %v, m15Signal=%v, h1Signal=%v, h4Signal=%v", symbol, m15Signal, h1Signal, h4Signal)
	return false
}

func (t *TrackerConsumerV3) checkSupertrendH4H1StochRsiUp(ctx context.Context, symbol eventmodels.StockSymbol) bool {
	ctx, span := otel.Tracer("tracker_v3_consumer").Start(ctx, "checkSupertrendH4H1StochRsiUp")
	defer span.End()

	logger := log.WithContext(ctx)

	state, unlock := t.GetState()
	defer unlock()

	m15SignalKey := fmt.Sprintf("%s-15-stochastic_rsi", symbol)
	m15Signal, found := state[m15SignalKey]
	if !found {
		logger.WithField("event", "signal").Warnf("checkSupertrendH4H1StochRsiUp: Signal not found: %s. Expected if signal was never received.", m15SignalKey)
		return false
	}

	h1SignalKey := fmt.Sprintf("%s-60-supertrend", symbol)
	h1Signal, found := state[h1SignalKey]
	if !found {
		logger.WithField("event", "signal").Warnf("checkSupertrendH4H1StochRsiUp: Signal not found: %s. Expected if signal was never received.", h1SignalKey)
		return false
	}

	h4SignalKey := fmt.Sprintf("%s-240-supertrend", symbol)
	h4Signal, found := state[h4SignalKey]
	if !found {
		logger.WithField("event", "signal").Warnf("checkSupertrendH4H1StochRsiUp: Signal not found: %s. Expected if signal was never received.", h4SignalKey)
		return false
	}

	if m15Signal == "buy" && h1Signal == "buy" && h4Signal == "buy" {
		logger.WithField("event", "signal").Infof("checkSupertrendH4H1StochRsiUp triggered for %v", symbol)
		return true
	}

	logger.WithField("event", "signal").Debugf("checkSupertrendH4H1StochRsiUp NOT triggered for %v, m15Signal=%v, h1Signal=%v, h4Signal=%v", symbol, m15Signal, h1Signal, h4Signal)
	return false
}

func (t *TrackerConsumerV3) checkSupertrendH1StochRsiUp(ctx context.Context, symbol eventmodels.StockSymbol) bool {
	ctx, span := otel.Tracer("tracker_v3_consumer").Start(ctx, "checkSupertrendH1StochRsiUp")
	defer span.End()

	logger := log.WithContext(ctx)

	state, unlock := t.GetState()
	defer unlock()

	m15SignalKey := fmt.Sprintf("%s-15-stochastic_rsi", symbol)
	m15Signal, found := state[m15SignalKey]
	if !found {
		logger.WithField("event", "signal").Warnf("checkSupertrendH1StochRsiUp: Signal not found: %s. Expected if signal was never received.", m15SignalKey)
		return false
	}

	h1SignalKey := fmt.Sprintf("%s-60-supertrend", symbol)
	h1Signal, found := state[h1SignalKey]
	if !found {
		logger.WithField("event", "signal").Warnf("checkSupertrendH1StochRsiUp: Signal not found: %s. Expected if signal was never received.", h1SignalKey)
		return false
	}

	if m15Signal == "buy" && h1Signal == "buy" {
		logger.WithField("event", "signal").Infof("checkSupertrendH1StochRsiUp triggered for %v", symbol)
		return true
	}

	logger.WithField("event", "signal").Debugf("checkSupertrendH1StochRsiUp NOT triggered for %v, m15Signal=%v, h1Signal=%v", symbol, m15Signal, h1Signal)
	return false
}

func (t *TrackerConsumerV3) updateState(ctx context.Context, event *eventmodels.TrackerV3) error {
	ctx, span := otel.Tracer("TrackerV3Consumer").Start(ctx, "updateState")
	defer span.End()

	logger := log.WithContext(ctx)

	state, unlock := t.GetState()
	defer unlock()

	components := strings.Split(event.SignalTracker.Name, "-")

	if len(components) != 2 {
		return fmt.Errorf("invalid SignalTracker name: %s", event.SignalTracker.Name)
	}

	signalName := components[0]
	signalValue := components[1]

	key := fmt.Sprintf("%s-%d-%s", event.SignalTracker.Header.Symbol, event.SignalTracker.Header.Timeframe, signalName)

	previousState := state[key]

	state[key] = signalValue

	if previousState != signalValue {
		logger.Infof("TrackerV3Consumer:updateState: updated state: key[%v] -> %v", key, signalValue)
	}

	return nil
}

func (t *TrackerConsumerV3) checkIsSignalTriggered(ctx context.Context, event *eventmodels.TrackerV3) []eventmodels.SignalTriggeredEvent {
	tracer := otel.Tracer("checkIsSignalTriggered")
	ctx, span := tracer.Start(ctx, "checkIsSignalTriggered")
	defer span.End()

	logger := log.WithContext(ctx)

	logger.Infof("TrackerV3Consumer:checkIsSignalTriggered: received terminal signal %s for %v", event.SignalTracker.Name, event.SignalTracker.Header.Symbol)

	triggeredEvents := make([]eventmodels.SignalTriggeredEvent, 0)

	switch event.SignalTracker.Name {
	case "stochastic_rsi-buy":
		// todo: implement a switch to check for different signals

		// if t.checkSupertrendH4H1StochRsiUp(ctx, event.SignalTracker.Header.Symbol) {
		// 	triggeredEvents = append(triggeredEvents, eventmodels.SignalTriggeredEvent{
		// 		Timestamp: event.SignalTracker.Timestamp,
		// 		Symbol:    event.SignalTracker.Header.Symbol,
		// 		Signal:    eventmodels.SuperTrend4h1hStochRsi15mUp,
		// 	})

		// 	logger.Info("SuperTrend4h1hStochRsi15mUp triggered")
		// }

		if t.checkSupertrendH1StochRsiUp(ctx, event.SignalTracker.Header.Symbol) {
			triggeredEvents = append(triggeredEvents, eventmodels.SignalTriggeredEvent{
				Timestamp: event.SignalTracker.Timestamp,
				Symbol:    event.SignalTracker.Header.Symbol,
				Signal:    eventmodels.SuperTrend1hStochRsi15mUp,
			})

			logger.Info("SuperTrend1hStochRsi15mUp triggered")
		}

	case "stochastic_rsi-sell":
		// todo: implement a switch to check for different signals

		// if t.checkSupertrendH4H1StochRsiDown(ctx, event.SignalTracker.Header.Symbol) {
		// 	triggeredEvents = append(triggeredEvents, eventmodels.SignalTriggeredEvent{
		// 		Timestamp: event.SignalTracker.Timestamp,
		// 		Symbol:    event.SignalTracker.Header.Symbol,
		// 		Signal:    eventmodels.SuperTrend4h1hStochRsi15mDown,
		// 	})

		// 	logger.Info("SuperTrend4h1hStochRsi15mDown triggered")
		// }

		if t.checkSupertrendH1StochRsiDown(ctx, event.SignalTracker.Header.Symbol) {
			triggeredEvents = append(triggeredEvents, eventmodels.SignalTriggeredEvent{
				Timestamp: event.SignalTracker.Timestamp,
				Symbol:    event.SignalTracker.Header.Symbol,
				Signal:    eventmodels.SuperTrend1hStochRsi15mDown,
			})

			logger.Info("SuperTrend1hStochRsi15mDown triggered")
		}

	default:
		logger.Infof("TrackerV3Consumer:checkIsSignalTriggered: received non-triggering event: %v for %v", event.SignalTracker, event.SignalTracker.Header.Symbol)
	}

	span.SetStatus(codes.Ok, "checkIsSignalTriggered completed")
	return triggeredEvents
}

type neverSampleSampler struct{}

func (ns neverSampleSampler) ShouldSample(p sdk_trace.SamplingParameters) sdk_trace.SamplingResult {
	return sdk_trace.SamplingResult{Decision: sdk_trace.Drop}
}

func (ns neverSampleSampler) Description() string {
	return "NeverSample"
}

func NeverSample() sdk_trace.Sampler {
	return neverSampleSampler{}
}

func (t *TrackerConsumerV3) processEvent(ctx context.Context, event EsdbEvent[*eventmodels.TrackerV3], processReplayEvents bool) error {
	var tracer trace.Tracer
	if event.IsReplay {
		tracerProvider := sdk_trace.NewTracerProvider(
			sdk_trace.WithSampler(NeverSample()),
		)
		tracer = tracerProvider.Tracer("tracker_v3_consumer")
	} else {
		if event.SpanContext.IsValid() {
			ctx = trace.ContextWithSpanContext(ctx, event.SpanContext)
		}

		tracer = otel.Tracer("tracker_v3_consumer")
	}

	// to make a link:
	// ctx, span = tracer.Start(ctx, "<- t.client.GetEventCh()", trace.WithLinks(trace.LinkFromContext(trace.ContextWithSpanContext(ctx, event.SpanContext))))

	ctx, span := tracer.Start(ctx, "<- t.client.GetEventCh()")
	defer span.End()

	logger := log.WithContext(ctx)

	ev := event.Event

	if ev.SignalTracker == nil {
		logger.Warnf("TrackerV3Consumer: received event without SignalTracker: %v", ev)
		return nil
	}

	if err := t.updateState(ctx, ev); err != nil {
		return fmt.Errorf("failed to update state: %v", err)
	}

	if event.IsReplay && !processReplayEvents {
		logger.Debugf("Ignore triggering replay event: %s", ev.SignalTracker.Name)
		return nil
	}

	triggeredEvents := t.checkIsSignalTriggered(ctx, ev)

	logger.Infof("Processing %v triggered events", len(triggeredEvents))

	for _, ev := range triggeredEvents {
		logger.Infof("Signal triggered: %s", ev.Symbol)
		t.signalTriggered <- eventmodels.SignalTriggeredEvent{
			Timestamp: ev.Timestamp,
			Symbol:    ev.Symbol,
			Signal:    ev.Signal,
			Ctx:       ctx,
		}
	}

	return nil
}

func (t *TrackerConsumerV3) Replay(ctx context.Context) {
	logger := log.WithContext(ctx)
	logger.Infof("Starting TrackerV3Consumer in replay mode")

	go func() {
		for event := range t.client.GetEventCh() {
			if err := t.processEvent(ctx, event, true); err != nil {
				logger.Errorf("TrackerV3Consumer.Replay: failed to process event: %v", err)
			}
		}

		fmt.Println("TrackerV3Consumer: replay done")
		t.CloseSignalTriggeredCh()
	}()

	t.client.Replay(ctx)

	logger.Infof("TrackerV3Consumer started in replay mode!!!")
}

func (t *TrackerConsumerV3) Start(ctx context.Context, processReplayEvents bool) {
	logger := log.WithContext(ctx)
	logger.Infof("Starting TrackerV3Consumer")

	go func() {
		for event := range t.client.GetEventCh() {
			ctx := context.Background()
			if err := t.processEvent(ctx, event, processReplayEvents); err != nil {
				logger.Errorf("TrackerV3Consumer: failed to process event: %v", err)
			}
		}
	}()

	t.client.Start(ctx)

	logger.Infof("TrackerV3Consumer started!!!")
}

func NewTrackerConsumerV3(client *TrackerV3Client) *TrackerConsumerV3 {
	return &TrackerConsumerV3{
		client:          client,
		state:           make(map[string]string),
		signalTriggered: make(chan eventmodels.SignalTriggeredEvent),
	}
}
