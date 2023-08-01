package slack

import (
	"fmt"
	"net/http"
	"slack-trading/src/eventpubsub"
)

func TradeApiHandler(w http.ResponseWriter, r *http.Request) {
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
