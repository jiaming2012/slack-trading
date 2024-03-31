package eventconsumers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventpubsub"
)

type OptionAlertWorker struct {
	wg                *sync.WaitGroup
	optionAlerts      []*eventmodels.OptionAlert
	brokerURL         string
	brokerBearerToken string
}

func (w *OptionAlertWorker) handleGetOptionAlertRequestEvent(event *eventmodels.GetOptionAlertRequestEvent) {
	log.Debugf("OptionAlertWorker.handleGetOptionAlertRequestEvent: %v", event)

	currentAlerts := []eventmodels.OptionAlert{}

	for _, alert := range w.optionAlerts {
		currentAlerts = append(currentAlerts, *alert)
	}

	eventpubsub.PublishCompletedResponse("OptionAlertWorker", &eventmodels.GetOptionAlertResponseEvent{
		Alerts: currentAlerts,
	}, &event.Meta)
}

func (w *OptionAlertWorker) handleCreateOptionAlertRequestEvent(event *eventmodels.CreateOptionAlertRequestEvent) {
	log.Debugf("OptionAlertWorker.handleCreateOptionAlertRequestEvent: %v", event)

	optionAlert, err := event.NewObject(event.ID)
	if err != nil {
		eventpubsub.PublishRequestError("OptionAlertWorker", err, &event.Meta)
		return
	}

	w.optionAlerts = append(w.optionAlerts, optionAlert)

	eventpubsub.PublishCompletedResponse("OptionAlertWorker", &eventmodels.CreateOptionAlertResponseEvent{
		ID: optionAlert.ID.String(),
	}, &event.Meta)
}

func (w *OptionAlertWorker) handleDeleteOptionAlertRequestEvent(event *eventmodels.DeleteOptionAlertRequestEvent) {
	log.Debugf("OptionAlertWorker.handleDeleteOptionAlertRequestEvent: %v", event)

	for i, alert := range w.optionAlerts {
		if alert.ID == event.AlertID {
			w.optionAlerts = append(w.optionAlerts[:i], w.optionAlerts[i+1:]...)
			break
		}
	}

	eventpubsub.PublishCompletedResponse("OptionAlertWorker", &eventmodels.DeleteOptionAlertResponseEvent{}, &event.Meta)
}

func (w *OptionAlertWorker) getSymbolList() string {
	var symbols strings.Builder

	for _, alert := range w.optionAlerts {
		symbols.WriteString(fmt.Sprintf("%s,", alert.OptionSymbol))
	}

	return symbols.String()
}

func (w *OptionAlertWorker) fetchOptionQuotes() (*eventmodels.OptionQuotesDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, w.brokerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("OptionAlertWorker:fetchOptionQuotes(): failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("symbols", w.getSymbolList())
	q.Add("greeks", "true")

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", w.brokerBearerToken))

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OptionAlertWorker:fetchOptionQuotes(): failed to fetch option prices: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OptionAlertWorker:fetchOptionQuotes(): failed to fetch option prices: %s", res.Status)
	}

	var optionQuotesDTO eventmodels.OptionQuotesDTO
	if err := json.NewDecoder(res.Body).Decode(&optionQuotesDTO); err != nil {
		return nil, fmt.Errorf("OptionAlertWorker:fetchOptionQuotes(): failed to decode json: %w", err)
	}

	return &optionQuotesDTO, nil
}

func (w *OptionAlertWorker) checkOptionAlerts(quoteMap eventmodels.OptionQuoteMap) []*eventmodels.OptionAlertUpdateEvent {
	var triggeredAlerts []*eventmodels.OptionAlertUpdateEvent
	for _, alert := range w.optionAlerts {
		if alert.TriggeredAt != nil {
			continue
		}

		if !alert.IsOptionActive {
			continue
		}

		quote, ok := quoteMap[alert.OptionSymbol]
		if !ok {
			log.Warnf("OptionAlertWorker:checkOptionAlerts(): option symbol not found: %s", alert.OptionSymbol)
			continue
		}

		var alertValue float64
		switch alert.AlertType {
		case eventmodels.LastPrice:
			alertValue = quote.LastPrice
		case eventmodels.Delta:
			alertValue = quote.Delta
		default:
			log.Errorf("OptionAlertWorker:checkOptionAlerts(): invalid alert type: %s", alert.AlertType.String())
			continue
		}

		switch alert.Condition.Type {
		case eventmodels.Cross:
			switch alert.Condition.Direction {
			case eventmodels.Above:
				if alertValue > alert.Condition.Value {
					triggeredAlerts = append(triggeredAlerts, &eventmodels.OptionAlertUpdateEvent{
						AlertID:      alert.ID,
						CreatedAt:    time.Now(),
						AlertMessage: fmt.Sprintf("Option %s %s crossed above %f", alert.OptionSymbol, alert.AlertType, alert.Condition.Value),
					})
				}
			case eventmodels.Below:
				if alertValue < alert.Condition.Value {
					triggeredAlerts = append(triggeredAlerts, &eventmodels.OptionAlertUpdateEvent{
						AlertID:      alert.ID,
						CreatedAt:    time.Now(),
						AlertMessage: fmt.Sprintf("Option %s %s crossed below %f", alert.OptionSymbol, alert.AlertType, alert.Condition.Value),
					})
				}
			default:
				log.Errorf("OptionAlertWorker:checkOptionAlerts(): invalid alert condition direction: %s", alert.Condition.Direction.String())
				continue
			}
		default:
			log.Errorf("OptionAlertWorker:checkOptionAlerts(): invalid alert condition type: %s", alert.Condition.Type.String())
			continue
		}
	}

	return triggeredAlerts
}

func (w *OptionAlertWorker) handleOptionAlertUpdate(event *eventmodels.OptionAlertUpdateEvent) {
	log.Debugf("OptionAlertWorker.handleOptionAlertUpdate: %v", event)

	var found bool
	for _, alert := range w.optionAlerts {
		if alert.ID == event.AlertID {
			alert.TriggeredAt = &event.CreatedAt
			found = true
			break
		}
	}

	if !found {
		log.Warnf("OptionAlertWorker.handleOptionAlertUpdate: alert not found: %s", event.AlertID)
	}

	eventpubsub.PublishCompletedResponse("OptionAlertWorker", &eventmodels.OptionAlertUpdateCompletedEvent{}, &event.Meta)
}

func (w *OptionAlertWorker) Start(ctx context.Context) {
	w.wg.Add(1)

	eventpubsub.Subscribe("OptionAlertWorker", eventmodels.GetOptionAlertRequestEventName, w.handleGetOptionAlertRequestEvent)
	eventpubsub.Subscribe("OptionAlertWorker", eventmodels.NewSavedEvent(eventmodels.CreateOptionAlertRequestEventName), w.handleCreateOptionAlertRequestEvent)
	eventpubsub.Subscribe("OptionAlertWorker", eventmodels.NewSavedEvent(eventmodels.DeleteOptionAlertRequestEventName), w.handleDeleteOptionAlertRequestEvent)
	eventpubsub.Subscribe("OptionAlertWorker", eventmodels.NewSavedEvent(eventmodels.OptionAlertUpdateEventName), w.handleOptionAlertUpdate)

	timer := time.NewTicker(15 * time.Second)

	go func() {
		defer w.wg.Done()

		for {
			select {
			case <-ctx.Done():
				log.Info("stopping OptionAlertWorker consumer")
				return
			case <-timer.C:
				optionPrices, err := w.fetchOptionQuotes()
				if err != nil {
					log.Errorf("OptionAlertWorker: failed to fetch option prices: %v", err)
					continue
				}

				quotes, _, err := optionPrices.ToModel()
				if err != nil {
					log.Errorf("OptionAlertWorker: failed to convert option prices to model: %v", err)
					continue
				}

				triggeredEvents := w.checkOptionAlerts(quotes)
				for _, event := range triggeredEvents {
					eventpubsub.PublishResponse("OptionAlertWorker", eventmodels.OptionAlertUpdateEventName, event, &eventmodels.MetaData{})
				}
			}
		}
	}()
}

func NewOptionAlertWorker(wg *sync.WaitGroup, brokerURL string, brokerBearerToken string) *OptionAlertWorker {
	return &OptionAlertWorker{
		wg:                wg,
		brokerURL:         brokerURL,
		brokerBearerToken: brokerBearerToken,
		optionAlerts:      []*eventmodels.OptionAlert{},
	}
}
