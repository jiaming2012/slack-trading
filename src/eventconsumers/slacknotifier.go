package eventconsumers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
)

type SlackNotifierClient struct {
	wg         *sync.WaitGroup
	webHookURL string
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

	_, err := sendResponse(msg, c.webHookURL, false)
	if err != nil {
		log.Error(err)
	}
}

func (c *SlackNotifierClient) executeOpenTradeResultHandler(ev *eventmodels.ExecuteOpenTradeResult) {
	log.Debugf("SlackNotifierClient.executeOpenTradeResultHandler <- %v", ev)

	msg := fmt.Sprintf("open trade: %v", ev.Trade)

	_, err := sendResponse(msg, c.webHookURL, false)
	if err != nil {
		log.Error(err)
	}
}

func (c *SlackNotifierClient) optionAlertUpdateEventHandler(ev *eventmodels.OptionAlertUpdateEvent) {
	log.Debugf("SlackNotifierClient.optionAlertUpdateEventHandler <- %v", ev)

	if _, err := sendResponse(ev.AlertMessage, c.webHookURL, false); err != nil {
		log.Errorf("SlackNotifierClient.optionAlertUpdateEventHandler: %v", err)
	}
}

func (c *SlackNotifierClient) tradierOrderDeleteEventHandler(ev *eventmodels.TradierOrderDeleteEvent) {
	log.Debugf("SlackNotifierClient.tradierOrderDeleteEventHandler <- %v", ev)

	msg := fmt.Sprintf("Order deleted -> ID: (%v)", ev.OrderID)

	if _, err := sendResponse(msg, c.webHookURL, false); err != nil {
		log.Errorf("SlackNotifierClient.tradierOrderDeleteEventHandler: %v", err)
	}
}

func (c *SlackNotifierClient) tradierOrderUpdateEventHandler(ev *eventmodels.TradierOrderUpdateEvent) {
	log.Debugf("SlackNotifierClient.tradierOrderUpdateEventHandler <- %v", ev)

	msg := fmt.Sprintf("Order updated -> ID (%v): [%v] %v -> %v", ev.OrderID, ev.Field, ev.Old, ev.New)

	if _, err := sendResponse(msg, c.webHookURL, false); err != nil {
		log.Errorf("SlackNotifierClient.tradierOrderUpdateEventHandler: %v", err)
	}
}

func (c *SlackNotifierClient) tradierOrderCreateEventHandler(ev *eventmodels.TradierOrderCreateEvent) {
	log.Debugf("SlackNotifierClient.optionOrderCreateEventHandler <- %v", ev)

	msg := fmt.Sprintf("Order created -> %v", ev.Order)

	if _, err := sendResponse(msg, c.webHookURL, false); err != nil {
		log.Errorf("SlackNotifierClient.optionOrderCreateEventHandler: %v", err)
	}
}

func (c *SlackNotifierClient) balanceResultHandler(balance eventmodels.Balance) {
	log.Debugf("SlackNotifierClient.sendBalance <- %v", balance)

	_, sendErr := sendResponse(balance.String(), c.webHookURL, false)
	if sendErr != nil {
		log.Errorf("SlackNotifierClient.sendBalance: %v", sendErr)
	}
}

func (c *SlackNotifierClient) sendError(err error) {
	log.Debugf("SlackNotifierClient.sendError <- %v", err)

	_, sendErr := sendResponse(err.Error(), c.webHookURL, false)
	if sendErr != nil {
		log.Errorf("SlackNotifierClient.sendError: %v", sendErr)
	}
}

func (c *SlackNotifierClient) sendTerminalError(err *eventmodels.TerminalError) {
	log.Debugf("SlackNotifierClient.sendError <- %v", err)

	if !err.GetMetaData().IsExternalRequest {
		return
	}

	_, sendErr := sendResponse(err.Error.Error(), c.webHookURL, false)
	if sendErr != nil {
		log.Errorf("SlackNotifierClient.sendError: %v", sendErr)
	}
}

func (c *SlackNotifierClient) getAccountsResponseHandler(ev *eventmodels.GetAccountsResponseEvent) {
	log.Debugf("SlackNotifierClient.getAccountsResponseHandler <- %v", ev.Accounts)
	meta := ev.GetMetaData()
	if meta.RequestID != uuid.Nil {
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

	_, sendErr := sendResponse(msg, c.webHookURL, false)
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

	_, sendErr := sendResponse(msg, c.webHookURL, false)
	if sendErr != nil {
		log.Error(sendErr)
	}
}

func (c *SlackNotifierClient) Start(ctx context.Context) {
	c.wg.Add(1)

	pubsub.Subscribe("SlackNotifierClient", eventmodels.AddAccountResponseEventEventName, c.addAccountResponseHandler)
	// pubsub.Subscribe("SlackNotifierClient", eventmodels.GetAccountsResponseEventName, c.getAccountsResponseHandler)
	pubsub.Subscribe("SlackNotifierClient", eventmodels.BalanceResultEventName, c.balanceResultHandler)
	pubsub.Subscribe("SlackNotifierClient", eventmodels.TradeFulfilledEventName, c.tradeFulfilledHandler)
	// pubsub.Subscribe("SlackNotifierClient", eventmodels.ExecuteOpenTradeResultEventName, c.executeOpenTradeResultHandler)
	pubsub.Subscribe("SlackNotifierClient", eventmodels.ExecuteCloseTradesResultEventName, c.executeCloseTradesResultHandler)
	pubsub.Subscribe("SlackNotifierClient", eventmodels.OptionAlertUpdateEventName, c.optionAlertUpdateEventHandler)
	pubsub.Subscribe("SlackNotifierClient", eventmodels.Error, c.sendError)
	pubsub.Subscribe("SlackNotifierClient", eventmodels.TradierOrderUpdateEventName, c.tradierOrderUpdateEventHandler)
	pubsub.Subscribe("SlackNotifierClient", eventmodels.TradierOrderDeleteEventName, c.tradierOrderDeleteEventHandler)
	pubsub.Subscribe("SlackNotifierClient", eventmodels.TradierOrderCreateEventName, c.tradierOrderCreateEventHandler)
	pubsub.Subscribe("SlackNotifierClient", eventmodels.TerminalErrorName, c.sendTerminalError)

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

func NewSlackNotifierClient(wg *sync.WaitGroup, webHookURL string) *SlackNotifierClient {
	return &SlackNotifierClient{
		wg:         wg,
		webHookURL: webHookURL,
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
		return nil, fmt.Errorf("PostJSON (Do): %w", getErr)
	}

	defer res.Body.Close()

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
