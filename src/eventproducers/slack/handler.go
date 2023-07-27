package slack

import (
	log "github.com/sirupsen/logrus"
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
		// todo: send event here
		log.Error(err)
		return
	}

	// todo: send responseURL as or in an event?
	log.Debugf("responseURL: %v", responseURL)

	switch cmd {
	case "/btc":
		tradeReq, validationErr := parseBTCRequest(r.Form)
		if validationErr != nil {
			log.Error(validationErr)
			// todo: send event here
			return
		}

		eventpubsub.Publish(eventpubsub.TradeRequestEvent, tradeReq)
	default:
		log.Errorf("Unknown cmd: %v", cmd)
		return
	}
}
