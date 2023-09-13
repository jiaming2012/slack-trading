package api

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
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
	if r.Method == "GET" {

	} else if r.Method == "POST" {
		var req eventmodels.OpenTradeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(400)
			return
		}

		if err := req.Validate(); err != nil {
			if respErr := SetErrorResponse("validation", 400, err, w); respErr != nil {
				log.Errorf("tradeHandler: failed to set error response: %v", respErr)
			}
			return
		}

		req.RequestID = uuid.New()
		resultCh, errCh := eventmodels.RegisterResultCallback(req.RequestID)

		pubsub.Publish("tradeHandler", pubsub.NewOpenTradeRequest, req)

		select {
		case result := <-resultCh:
			res, ok := result.(*eventmodels.ExecuteOpenTradeResult)
			if !ok {
				log.Errorf("tradeHandler: failed to read ExecuteOpenTradeResult")
				return
			}

			if err := SetResponse(res, w); err != nil {
				log.Errorf("tradeHandler: failed to set response: %v", err)
			}
		case err := <-errCh:
			if respErr := SetErrorResponse("req", 400, err, w); respErr != nil {
				log.Errorf("tradeHandler: failed to set error response: %v", respErr)
			}
		}
	} else {
		w.WriteHeader(404)
	}
}

func SetupApiHandler(router *mux.Router) {
	router.HandleFunc("", tradeHandler)
	router.HandleFunc("/signal", signalHandler)
}

//func NewApiHandler(dispatcher *eventmodels.globalDispatcher) *apiHandler {
//	handler := &apiHandler{
//		dispatcher: dispatcher,
//	}
//
//	return handler
//}
