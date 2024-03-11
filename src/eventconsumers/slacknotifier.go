package eventconsumers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
)

// todo: add config
var (
	WebhookURL = os.Getenv("WEBHOOK_URL")
)

type SlackNotifierClient struct {
	wg *sync.WaitGroup
}

// tradeFulfilledHandler: todo: remove - deprecated
func (c *SlackNotifierClient) tradeFulfilledHandler(ev eventmodels.TradeFulfilledEvent) {
	log.Debugf("SlackNotifierClient.sendTradeConfirmation <- %v", ev)

	msg := fmt.Sprintf("%.2f btc @%.8f successfully placed", ev.Volume, ev.ExecutedPrice)

	_, err := sendResponse(msg, ev.ResponseURL, false)
	if err != nil {
		log.Error(err)
	}
}

func (c *SlackNotifierClient) executeCloseTradesResultHandler(ev *eventmodels.ExecuteCloseTradesResult) {
	log.Debugf("SlackNotifierClient.executeCloseTradesResultHandler <- %v", ev)

	msg := fmt.Sprintf("close trade: %v", ev.Trade)

	_, err := sendResponse(msg, WebhookURL, false)
	if err != nil {
		log.Error(err)
	}
}

func (c *SlackNotifierClient) executeOpenTradeResultHandler(ev *eventmodels.ExecuteOpenTradeResult) {
	log.Debugf("SlackNotifierClient.executeOpenTradeResultHandler <- %v", ev)

	msg := fmt.Sprintf("open trade: %v", ev.Trade)

	_, err := sendResponse(msg, WebhookURL, false)
	if err != nil {
		log.Error(err)
	}
}

func (c *SlackNotifierClient) balanceResultHandler(balance eventmodels.Balance) {
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

func (c *SlackNotifierClient) getAccountsResponseHandler(ev *eventmodels.GetAccountsResponseEvent) {
	log.Debugf("SlackNotifierClient.getAccountsResponseHandler <- %v", ev.Accounts)
	if ev.GetRequestID() != uuid.Nil {
		log.Debugf("SlackNotifierClient.getAccountsResponseHandler: ignore requests that have a request id")
		return
	}

	var msg string
	if len(ev.Accounts) == 0 {
		msg = "No accounts available"
	} else {
		var str strings.Builder

		str.WriteString("** Accounts **\n")
		str.WriteString("------------------------\n")

		for i, account := range ev.Accounts {
			str.WriteString(fmt.Sprintf("%d: %v\n", i+1, account.String()))
		}

		msg = str.String()
	}

	_, sendErr := sendResponse(msg, WebhookURL, false)
	if sendErr != nil {
		log.Error(sendErr)
	}
}

func (c *SlackNotifierClient) addAccountResponseHandler(ev eventmodels.AddAccountResponseEvent) {
	log.Debugf("SlackNotifierClient.addAccountResponseHandler <- %v", ev.Account)

	// todo: this condition should be determined by a source field on the request
	if ev.RequestID != uuid.Nil {
		log.Debugf("SlackNotifierClient.addAccountResponseHandler: ignore requests that have a request id")
		return
	}

	msg := fmt.Sprintf("Successfully added account:\n%v", ev.Account.String())

	_, sendErr := sendResponse(msg, WebhookURL, false)
	if sendErr != nil {
		log.Error(sendErr)
	}
}

func (c *SlackNotifierClient) Start(ctx context.Context) {
	c.wg.Add(1)

	pubsub.Subscribe("SlackNotifierClient", pubsub.AddAccountResponseEvent, c.addAccountResponseHandler)
	pubsub.Subscribe("SlackNotifierClient", pubsub.GetAccountsResponseEvent, c.getAccountsResponseHandler)
	pubsub.Subscribe("SlackNotifierClient", pubsub.BalanceResultEvent, c.balanceResultHandler)
	pubsub.Subscribe("SlackNotifierClient", pubsub.TradeFulfilledEvent, c.tradeFulfilledHandler)
	pubsub.Subscribe("SlackNotifierClient", pubsub.ExecuteOpenTradeResult, c.executeOpenTradeResultHandler)
	pubsub.Subscribe("SlackNotifierClient", pubsub.ExecuteCloseTradesResult, c.executeCloseTradesResultHandler)
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

type block map[string]interface{}

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
		var errDTO eventmodels.ErrorDTO
		if jsonErr := json.Unmarshal(bodyBytes, &errDTO); jsonErr != nil {
			return nil, fmt.Errorf("PostJSON (jsonErr): %w. payload: %s", jsonErr, string(bodyBytes))
		}

		return nil, fmt.Errorf("errDTO.Msg: %v", errDTO.Msg)
	}

	return bodyBytes, nil
}
