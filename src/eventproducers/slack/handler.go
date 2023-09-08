package slack

import (
	"fmt"
	"net/http"
	"slack-trading/src/eventmodels"
	"slack-trading/src/eventpubsub"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	// todo: should only be called from main slack handler
	// it should be clear that the handler is from the trades channel in slack

	// Immediately return 200 back to the slack server. Slack gives apps 3 seconds to return a
	// response. Otherwise, it is expected that the app will use the response_url in the request
	// to reply asynchronously.
	w.WriteHeader(200)

	r.ParseForm()

	cmd, responseURL, err := validateForm(r.Form)
	if err != nil {
		eventpubsub.PublishError("TradeApiHandler/validateForm", err)
		return
	}

	switch cmd {
	case "/accounts":
		request, validationErr := parseAccountRequest(r.Form)
		if validationErr != nil {
			eventpubsub.PublishError("TradeApiHandler/accounts", validationErr)
			return
		}

		switch event := request.(type) {
		case eventmodels.AddAccountRequestEvent:
			eventpubsub.Publish("TradeApiHandler/accounts", eventpubsub.AddAccountRequestEvent, event)
		case eventmodels.GetAccountsRequestEvent:
			eventpubsub.Publish("TradeApiHandler/accounts", eventpubsub.GetAccountsRequestEvent, event)
		default:
			eventpubsub.PublishError("TradeApiHandler/accounts", fmt.Errorf("unknown request type: %T", request))
		}

	case "/balance":
		symbol, validationErr := parseBalanceRequest(r.Form)
		if validationErr != nil {
			eventpubsub.PublishError("TradeApiHandler/balance", validationErr)
			return
		}

		eventpubsub.Publish("TradeApiHandler/balance", eventpubsub.BalanceRequestEvent, symbol)

	case "/btc":
		tradeReq, validationErr := parseBTCTradeRequest(r.Form)
		if validationErr != nil {
			eventpubsub.PublishError("TradeApiHandler/btc", validationErr)
			return
		}

		tradeReq.ResponseURL = responseURL
		eventpubsub.Publish("TradeApiHandler/btc", eventpubsub.TradeRequestEvent, tradeReq)
	default:
		cmdErr := fmt.Errorf("unknown cmd: %v", cmd)
		eventpubsub.PublishError("TradeApiHandler/cmd", cmdErr)
		return
	}
}
