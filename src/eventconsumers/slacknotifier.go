package eventconsumers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	models "slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	"sync"
	"time"
)

// todo: add config
const (
	WebhookURL = "https://hooks.slack.com/services/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"
)

type SlackNotifierClient struct {
	wg *sync.WaitGroup
}

func (c *SlackNotifierClient) sendTradeConfirmation(ev models.TradeFulfilledEvent) {
	log.Debugf("SlackNotifierClient.sendTradeConfirmation <- %v", ev)

	msg := fmt.Sprintf("%.2f btc @%.8f successfully placed", ev.Volume, ev.ExecutedPrice)

	_, err := sendResponse(msg, ev.ResponseURL, false)
	if err != nil {
		log.Error(err)
	}
}

func (c *SlackNotifierClient) sendBalance(balance models.Balance) {
	log.Debugf("SlackNotifierClient.sendBalance <- %v", balance)

	_, sendErr := sendResponse(balance.String(), WebhookURL, false)
	if sendErr != nil {
		log.Error(sendErr)
	}
}

func (c *SlackNotifierClient) sendError(err error) {
	log.Debugf("SlackNotifierClient.sendError <- %v", err)

	_, sendErr := sendResponse(err.Error(), WebhookURL, false)
	if sendErr != nil {
		log.Error(sendErr)
	}
}

func (c *SlackNotifierClient) Start(ctx context.Context) {
	c.wg.Add(1)

	pubsub.Subscribe("SlackNotifierClient", pubsub.BalanceResultEvent, c.sendBalance)
	pubsub.Subscribe("SlackNotifierClient", pubsub.TradeFulfilledEvent, c.sendTradeConfirmation)
	pubsub.Subscribe("SlackNotifierClient", pubsub.Error, c.sendError)

	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-ctx.Done():
				log.Info("stopping SlackNotifierClient consumer")
				return
			}
		}
	}()
}

func NewSlackNotifierClient(wg *sync.WaitGroup) *SlackNotifierClient {
	return &SlackNotifierClient{
		wg: wg,
	}
}

func sendResponse(msg string, url string, isEphemeral bool) ([]byte, error) {
	body := make(map[string]interface{})
	body["text"] = msg
	if isEphemeral {
		body["response_type"] = "ephemeral"
	} else {
		body["response_type"] = "in_channel"
	}

	return postJSON(url, body)
}

func postJSON(url string, body map[string]interface{}) ([]byte, error) {
	client := http.Client{
		Timeout: 60 * time.Second,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("PostJSON (Marshal): %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("PostJSON (NewRequest): %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, getErr := client.Do(req)
	if getErr != nil {
		return nil, fmt.Errorf("PostJSON (Do): %w", err)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	bodyBytes, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return nil, fmt.Errorf("PostJSON (ReadAll): %w", readErr)
	}

	if res.StatusCode >= 400 {
		var errDTO models.ErrorDTO
		if jsonErr := json.Unmarshal(bodyBytes, &errDTO); jsonErr != nil {
			return nil, fmt.Errorf("PostJSON (jsonErr): %w", jsonErr)
		}

		return nil, fmt.Errorf("errDTO.Msg: %v", errDTO.Msg)
	}

	return bodyBytes, nil
}
