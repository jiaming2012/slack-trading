package tradeapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
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

// func getTradesByAccountHandler(w http.ResponseWriter, r *http.Request) {
// 	if r.Method == "GET" {
// 		vars := mux.Vars(r)
// 		accountName, found := vars["accountName"]
// 		if !found {
// 			err := fmt.Errorf("could not find accountName in request params")
// 			if respErr := eventproducers.SetErrorResponse("validation", 400, err, w); respErr != nil {
// 				log.Errorf("getTradesByAccountHandler: failed to set error response: %v", respErr)
// 			}
// 			return
// 		}

// 		req := eventmodels.NewFetchTradesRequest(uuid.New(), accountName, nil)

// 		resultCh, errCh := eventmodels.RegisterResultCallback(req.RequestID)

// 		pubsub.Publish("getTradesByAccountHandler", pubsub.FetchTradesRequest, req)

// 		select {
// 		case result := <-resultCh:
// 			res, ok := result.(*eventmodels.FetchTradesResult)
// 			if !ok {
// 				log.Errorf("getTradesByAccountHandler: failed to read FetchTradesResult")
// 				return
// 			}

// 			if err := eventproducers.SetResponse(res, w); err != nil {
// 				log.Errorf("getTradesByAccountHandler: failed to set response: %v", err)
// 				w.WriteHeader(500)
// 				return
// 			}
// 		case err := <-errCh:
// 			if respErr := eventproducers.SetErrorResponse("req", 400, err, w); respErr != nil {
// 				log.Errorf("getTradesByAccountHandler: failed to set error response: %v", respErr)
// 				w.WriteHeader(500)
// 				return
// 			}
// 		}
// 	} else {
// 		w.WriteHeader(404)
// 	}
// }

// func closeTradeHandler(w http.ResponseWriter, r *http.Request) {
// 	if r.Method == "POST" {
// 		var req eventmodels.CloseTradeRequest
// 		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 			w.WriteHeader(400)
// 			return
// 		}

// 		if err := req.Validate(); err != nil {
// 			if respErr := eventproducers.SetErrorResponse("validation", 400, err, w); respErr != nil {
// 				log.Errorf("tradeHandler: failed to set error response: %v", respErr)
// 				w.WriteHeader(500)
// 				return
// 			}
// 			return
// 		}

// 		req.RequestID = uuid.New()
// 		resultCh, errCh := eventmodels.RegisterResultCallback(req.RequestID)

// 		pubsub.Publish("closeTradeHandler", pubsub.CloseTradesRequest, &req)

// 		select {
// 		case result := <-resultCh:
// 			res, ok := result.(*eventmodels.ExecuteCloseTradesResult)
// 			if !ok {
// 				log.Errorf("closeTradeHandler: failed to read ExecuteOpenTradeResult")
// 				return
// 			}

// 			if err := eventproducers.SetResponse(res, w); err != nil {
// 				log.Errorf("closeTradeHandler: failed to set response: %v", err)
// 				w.WriteHeader(500)
// 				return
// 			}
// 		case err := <-errCh:
// 			if respErr := eventproducers.SetErrorResponse("req", 400, err, w); respErr != nil {
// 				log.Errorf("closeTradeHandler: failed to set error response: %v", respErr)
// 				w.WriteHeader(500)
// 				return
// 			}
// 		}
// 	} else {
// 		w.WriteHeader(404)
// 	}
// }

// func openTradeHandler(w http.ResponseWriter, r *http.Request) {
// 	if r.Method == "POST" {
// 		var req eventmodels.OpenTradeRequest
// 		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 			w.WriteHeader(400)
// 			return
// 		}

// 		if err := req.Validate(); err != nil {
// 			if respErr := eventproducers.SetErrorResponse("validation", 400, err, w); respErr != nil {
// 				log.Errorf("openTradeHandler: failed to set error response: %v", respErr)
// 			}
// 			return
// 		}

// 		req.RequestID = uuid.New()
// 		resultCh, errCh := eventmodels.RegisterResultCallback(req.RequestID)

// 		pubsub.Publish("openTradeHandler", pubsub.NewOpenTradeRequest, req)

// 		select {
// 		case result := <-resultCh:
// 			res, ok := result.(*eventmodels.ExecuteOpenTradeResult)
// 			if !ok {
// 				log.Errorf("openTradeHandler: failed to read ExecuteOpenTradeResult")
// 				w.WriteHeader(500)
// 				return
// 			}

// 			if err := eventproducers.SetResponse(res, w); err != nil {
// 				log.Errorf("openTradeHandler: failed to set response: %v", err)
// 				w.WriteHeader(500)
// 				return
// 			}
// 		case err := <-errCh:
// 			if respErr := eventproducers.SetErrorResponse("req", 400, err, w); respErr != nil {
// 				log.Errorf("openTradeHandler: failed to set error response: %v", respErr)
// 				w.WriteHeader(500)
// 				return
// 			}
// 		}
// 	} else {
// 		w.WriteHeader(404)
// 	}
// }

func handleOpenTrade(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		eventproducers.ApiRequestHandler(pubsub.OpenTradeRequest, &eventmodels.OpenTradeRequest{}, &eventmodels.ExecuteOpenTradeResult{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func handleCloseTrade(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		eventproducers.ApiRequestHandler(pubsub.CloseTradeRequest, &eventmodels.CloseTradeRequest{}, &eventmodels.ExecuteCloseTradesResult{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func handleTradesByAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		eventproducers.ApiRequestHandler(pubsub.FetchTradesRequest, &eventmodels.FetchTradesRequest{}, &eventmodels.FetchTradesResult{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func SetupHandler(router *mux.Router) {
	router.HandleFunc("", handleOpenTrade)
	router.HandleFunc("/close", handleCloseTrade)
	router.HandleFunc("/account/{accountName}", handleTradesByAccount)
	router.HandleFunc("/signal", signalHandler)
}
