package tradeapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
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

func handleOpenTrade(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		eventproducers.ApiRequestHandler(eventmodels.OpenTradeRequestEventName, &eventmodels.CreateTradeRequest{}, &eventmodels.ExecuteOpenTradeResult{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func handleCloseTrade(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		eventproducers.ApiRequestHandler(eventmodels.CloseTradeRequestEventName, &eventmodels.CloseTradeRequest{}, &eventmodels.ExecuteCloseTradesResult{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func handleTradesByAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		eventproducers.ApiRequestHandler(eventmodels.FetchTradesRequestEventName, &eventmodels.FetchTradesRequest{}, &eventmodels.FetchTradesResult{}, w, r)
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
