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

func (t *TrackerV3Consumer) checkSupertrendH1H4StochRsiUp(symbol eventmodels.StockSymbol) bool {
	m15SignalKey := fmt.Sprintf("%s-15-stochastic_rsi", symbol)
	m15Signal, found := t.state[m15SignalKey]
	if !found {
		log.Warnf("Signal not found: %s. Expected if signal was never received.", m15SignalKey)
		return false
	}

	h1SignalKey := fmt.Sprintf("%s-60-supertrend", symbol)
	h1Signal, found := t.state[h1SignalKey]
	if !found {
		log.Warnf("Signal not found: %s. Expected if signal was never received.", h1SignalKey)
		return false
	}

	h4SignalKey := fmt.Sprintf("%s-240-supertrend", symbol)
	h4Signal, found := t.state[h4SignalKey]
	if !found {
		log.Warnf("Signal not found: %s. Expected if signal was never received.", h4SignalKey)
		return false
	}

	if m15Signal == "sell" && h1Signal == "buy" && h4Signal == "buy" {
		return true
	}

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

func (t *TrackerV3Consumer) checkIsSignalTriggered(event *eventmodels.TrackerV3) (bool, eventmodels.SignalName) {
	switch event.SignalTracker.Name {
	case "stochastic_rsi-sell":
		return t.checkSupertrendH1H4StochRsiUp(event.SignalTracker.Header.Symbol), eventmodels.SuperTrend4h1hStochRsi15mUp
	default:
		log.Infof("TrackerV3Consumer:checkIsSignalTriggered: non-triggering event received: %s", event.SignalTracker.Name)
		return false, ""
	}
}

func (t *TrackerV3Consumer) Start(ctx context.Context) {
	log.Infof("Starting TrackerV3Consumer")

	go func() {
		for event := range t.client.GetEventCh() {
			if event.SignalTracker != nil {
				if err := t.updateState(event); err != nil {
					log.Errorf("Failed to update state: %v", err)
					continue
				}

				if isTriggered, signalName := t.checkIsSignalTriggered(event); isTriggered {
					log.Infof("Signal triggered: %s", event.SignalTracker.Name)
					t.signalTriggered <- SignalTriggeredEvent{
						Symbol: event.SignalTracker.Header.Symbol,
						Signal: signalName,
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
