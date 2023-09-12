package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
)

func signalHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(400)
			return
		}

		fmt.Println(body)
	}
}

func tradeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var openTradeRequest eventmodels.OpenTradeRequest
		if err := json.NewDecoder(r.Body).Decode(&openTradeRequest); err != nil {
			w.WriteHeader(400)
			return
		}

		if err := openTradeRequest.Validate(); err != nil {
			if respErr := SetErrorResponse("validation", 400, err, w); respErr != nil {
				log.Errorf("tradeHandler: failed to set error response: %v", respErr)
			}
			return
		}

		openTradeRequest.Result = make(chan *eventmodels.ExecuteOpenTradeResult)
		openTradeRequest.Error = make(chan error)

		pubsub.Publish("tradeHandler", pubsub.NewOpenTradeRequest, openTradeRequest)

		select {
		case result := <-openTradeRequest.Result:
			if err := SetResponse(result, w); err != nil {
				log.Errorf("tradeHandler: failed to set response: %v", err)
			}
		case err := <-openTradeRequest.Error:
			if respErr := SetErrorResponse("openTradeRequest", 400, err, w); respErr != nil {
				log.Errorf("tradeHandler: failed to set error response: %v", respErr)
			}
		}
	}
}

func TradesHandler(router *mux.Router) {
	router.HandleFunc("", tradeHandler)
	router.HandleFunc("/signal", signalHandler)
}
