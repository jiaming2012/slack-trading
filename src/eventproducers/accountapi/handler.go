package accountapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
	pubsub "slack-trading/src/eventpubsub"
)

func handleAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		eventproducers.ApiRequestHandler(pubsub.GetAccountsRequestEvent, &eventmodels.GetAccountsRequestEvent{}, &eventmodels.GetAccountsResponseEvent{}, w, r)
	} else if r.Method == "POST" {
		eventproducers.ApiRequestHandler(pubsub.CreateAccountRequestEvent, &eventmodels.CreateAccountRequestEvent{}, &eventmodels.CreateAccountResponseEvent{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func handleStrategies(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.WriteHeader(404)
	} else if r.Method == "POST" {
		eventproducers.ApiRequestHandler(pubsub.CreateAccountStrategyRequestEvent, &eventmodels.CreateAccountStrategyRequestEvent{}, &eventmodels.CreateAccountStrategyResponseEvent{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func handleAccountStats(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		eventproducers.ApiRequestHandler(pubsub.NewGetStatsRequest, &eventmodels.GetStatsRequest{}, &eventmodels.GetStatsResult{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func SetupHandler(router *mux.Router) {
	router.HandleFunc("", handleAccounts)
	router.HandleFunc("/{accountName}/stats", handleAccountStats)
	router.HandleFunc("/{accountName}/strategies", handleStrategies)
}
