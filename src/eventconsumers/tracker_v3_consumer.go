package eventconsumers

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
)

type TrackerV3Client = esdbConsumerStream[*eventmodels.TrackerV3]

type TrackerV3Consumer struct {
	client          *TrackerV3Client
	state           map[string]string
	signalTriggered chan SignalTriggeredEvent
}

func (t *TrackerV3Consumer) GetState() map[string]string {
	return t.state
}

type SignalTriggeredEvent struct {
	Symbol eventmodels.StockSymbol
	Signal eventmodels.SignalName
}

func (t *TrackerV3Consumer) GetSignalTriggeredCh() <-chan SignalTriggeredEvent {
	return t.signalTriggered
}

func (t *TrackerV3Consumer) checkSupertrendH1StochRsiDown(symbol eventmodels.StockSymbol) bool {
	m15SignalKey := fmt.Sprintf("%s-15-stochastic_rsi", symbol)
	m15Signal, found := t.state[m15SignalKey]
	if !found {
		log.WithField("event", "signal").Warnf("checkSupertrendH1StochRsiDown: Signal not found: %s. Expected if signal was never received.", m15SignalKey)
		return false
	}

	h1SignalKey := fmt.Sprintf("%s-60-supertrend", symbol)
	h1Signal, found := t.state[h1SignalKey]
	if !found {
		log.WithField("event", "signal").Warnf("checkSupertrendH1StochRsiDown: Signal not found: %s. Expected if signal was never received.", h1SignalKey)
		return false
	}

	if m15Signal == "sell" && h1Signal == "sell" {
		log.WithField("event", "signal").Infof("checkSupertrendH1StochRsiDown: triggered for %v", symbol)
		return true
	}

	log.WithField("event", "signal").Debugf("checkSupertrendH1StochRsiDown NOT triggered for %v, m15Signal=%v, h1Signal=%v", symbol, m15Signal, h1Signal)
	return false
}

func (t *TrackerV3Consumer) checkSupertrendH4H1StochRsiDown(symbol eventmodels.StockSymbol) bool {
	m15SignalKey := fmt.Sprintf("%s-15-stochastic_rsi", symbol)
	m15Signal, found := t.state[m15SignalKey]
	if !found {
		log.WithField("event", "signal").Warnf("checkSupertrendH4H1StochRsiDown: Signal not found: %s. Expected if signal was never received.", m15SignalKey)
		return false
	}

	h1SignalKey := fmt.Sprintf("%s-60-supertrend", symbol)
	h1Signal, found := t.state[h1SignalKey]
	if !found {
		log.WithField("event", "signal").Warnf("checkSupertrendH4H1StochRsiDown: Signal not found: %s. Expected if signal was never received.", h1SignalKey)
		return false
	}

	h4SignalKey := fmt.Sprintf("%s-240-supertrend", symbol)
	h4Signal, found := t.state[h4SignalKey]
	if !found {
		log.WithField("event", "signal").Warnf("checkSupertrendH4H1StochRsiDown: Signal not found: %s. Expected if signal was never received.", h4SignalKey)
		return false
	}

	if m15Signal == "sell" && h1Signal == "sell" && h4Signal == "sell" {
		log.WithField("event", "signal").Infof("checkSupertrendH4H1StochRsiDown: triggered for %v", symbol)
		return true
	}

	log.WithField("event", "signal").Debugf("checkSupertrendH4H1StochRsiDown NOT triggered for %v, m15Signal=%v, h1Signal=%v, h4Signal=%v", symbol, m15Signal, h1Signal, h4Signal)
	return false
}

func (t *TrackerV3Consumer) checkSupertrendH4H1StochRsiUp(symbol eventmodels.StockSymbol) bool {
	m15SignalKey := fmt.Sprintf("%s-15-stochastic_rsi", symbol)
	m15Signal, found := t.state[m15SignalKey]
	if !found {
		log.WithField("event", "signal").Warnf("checkSupertrendH4H1StochRsiUp: Signal not found: %s. Expected if signal was never received.", m15SignalKey)
		return false
	}

	h1SignalKey := fmt.Sprintf("%s-60-supertrend", symbol)
	h1Signal, found := t.state[h1SignalKey]
	if !found {
		log.WithField("event", "signal").Warnf("checkSupertrendH4H1StochRsiUp: Signal not found: %s. Expected if signal was never received.", h1SignalKey)
		return false
	}

	h4SignalKey := fmt.Sprintf("%s-240-supertrend", symbol)
	h4Signal, found := t.state[h4SignalKey]
	if !found {
		log.WithField("event", "signal").Warnf("checkSupertrendH4H1StochRsiUp: Signal not found: %s. Expected if signal was never received.", h4SignalKey)
		return false
	}

	if m15Signal == "buy" && h1Signal == "buy" && h4Signal == "buy" {
		log.WithField("event", "signal").Infof("checkSupertrendH4H1StochRsiUp triggered for %v", symbol)
		return true
	}

	log.WithField("event", "signal").Debugf("checkSupertrendH4H1StochRsiUp NOT triggered for %v, m15Signal=%v, h1Signal=%v, h4Signal=%v", symbol, m15Signal, h1Signal, h4Signal)
	return false
}

func (t *TrackerV3Consumer) checkSupertrendH1StochRsiUp(symbol eventmodels.StockSymbol) bool {
	m15SignalKey := fmt.Sprintf("%s-15-stochastic_rsi", symbol)
	m15Signal, found := t.state[m15SignalKey]
	if !found {
		log.WithField("event", "signal").Warnf("checkSupertrendH1StochRsiUp: Signal not found: %s. Expected if signal was never received.", m15SignalKey)
		return false
	}

	h1SignalKey := fmt.Sprintf("%s-60-supertrend", symbol)
	h1Signal, found := t.state[h1SignalKey]
	if !found {
		log.WithField("event", "signal").Warnf("checkSupertrendH1StochRsiUp: Signal not found: %s. Expected if signal was never received.", h1SignalKey)
		return false
	}

	if m15Signal == "buy" && h1Signal == "buy" {
		log.WithField("event", "signal").Infof("checkSupertrendH1StochRsiUp triggered for %v", symbol)
		return true
	}

	log.WithField("event", "signal").Debugf("checkSupertrendH1StochRsiUp NOT triggered for %v, m15Signal=%v, h1Signal=%v", symbol, m15Signal, h1Signal)
	return false
}

func (t *TrackerV3Consumer) updateState(event *eventmodels.TrackerV3) error {
	components := strings.Split(event.SignalTracker.Name, "-")

	if len(components) != 2 {
		return fmt.Errorf("invalid SignalTracker name: %s", event.SignalTracker.Name)
	}

	signalName := components[0]
	signalValue := components[1]

	key := fmt.Sprintf("%s-%d-%s", event.SignalTracker.Header.Symbol, event.SignalTracker.Header.Timeframe, signalName)

	t.state[key] = signalValue

	return nil
}

func (t *TrackerV3Consumer) checkIsSignalTriggered(event *eventmodels.TrackerV3) []SignalTriggeredEvent {
	log.Infof("TrackerV3Consumer:checkIsSignalTriggered: received terminal signal %s for %v", event.SignalTracker.Name, event.SignalTracker.Header.Symbol)

	triggeredEvents := make([]SignalTriggeredEvent, 0)

	switch event.SignalTracker.Name {
	case "stochastic_rsi-buy":
		if t.checkSupertrendH4H1StochRsiUp(event.SignalTracker.Header.Symbol) {
			triggeredEvents = append(triggeredEvents, SignalTriggeredEvent{
				Symbol: event.SignalTracker.Header.Symbol,
				Signal: eventmodels.SuperTrend4h1hStochRsi15mUp,
			})
		}

		if t.checkSupertrendH1StochRsiUp(event.SignalTracker.Header.Symbol) {
			triggeredEvents = append(triggeredEvents, SignalTriggeredEvent{
				Symbol: event.SignalTracker.Header.Symbol,
				Signal: eventmodels.SuperTrend1hStochRsi15mUp,
			})
		}

	case "stochastic_rsi-sell":
		if t.checkSupertrendH4H1StochRsiDown(event.SignalTracker.Header.Symbol) {
			triggeredEvents = append(triggeredEvents, SignalTriggeredEvent{
				Symbol: event.SignalTracker.Header.Symbol,
				Signal: eventmodels.SuperTrend4h1hStochRsi15mDown,
			})
		}

		if t.checkSupertrendH1StochRsiDown(event.SignalTracker.Header.Symbol) {
			triggeredEvents = append(triggeredEvents, SignalTriggeredEvent{
				Symbol: event.SignalTracker.Header.Symbol,
				Signal: eventmodels.SuperTrend1hStochRsi15mDown,
			})
		}

	default:
		log.Infof("TrackerV3Consumer:checkIsSignalTriggered: received non-triggering event: %v for %v", event.SignalTracker, event.SignalTracker.Header.Symbol)
	}

	return triggeredEvents
}

func (t *TrackerV3Consumer) Start(ctx context.Context) {
	log.Infof("Starting TrackerV3Consumer")

	go func() {
		for event := range t.client.GetEventCh() {
			ev := event.Event

			if ev.SignalTracker != nil {
				if err := t.updateState(ev); err != nil {
					log.Errorf("Failed to update state: %v", err)
					continue
				}

				if event.IsReplay {
					log.Debugf("Ignore triggering replay event: %s", ev.SignalTracker.Name)
					continue
				}

				triggeredEvents := t.checkIsSignalTriggered(ev)

				log.Infof("Processing %v triggered events", len(triggeredEvents))

				for _, ev := range triggeredEvents {
					log.Infof("Signal triggered: %s", ev.Symbol)
					t.signalTriggered <- SignalTriggeredEvent{
						Symbol: ev.Symbol,
						Signal: ev.Signal,
					}
				}
			}
		}
	}()

	t.client.Start(ctx)

	log.Infof("TrackerV3Consumer started!!!")
}

func NewTrackerV3Consumer(client *TrackerV3Client) *TrackerV3Consumer {
	return &TrackerV3Consumer{
		client:          client,
		state:           make(map[string]string),
		signalTriggered: make(chan SignalTriggeredEvent),
	}
}
