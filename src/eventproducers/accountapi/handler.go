package accountapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
)

func handleAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		eventproducers.ApiRequestHandler(eventmodels.GetAccountsRequestEventName, &eventmodels.GetAccountsRequestEvent{}, &eventmodels.GetAccountsResponseEvent{}, w, r)
	} else if r.Method == "POST" {
		eventproducers.ApiRequestHandler(eventmodels.CreateAccountRequestEventName, &eventmodels.CreateAccountRequestEvent{}, &eventmodels.CreateAccountResponseEvent{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func handleStrategies(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.WriteHeader(404)
	} else if r.Method == "POST" {
		eventproducers.ApiRequestHandler(eventmodels.CreateAccountStrategyRequestEventName, &eventmodels.CreateAccountStrategyRequestEvent{}, &eventmodels.CreateAccountStrategyResponseEvent{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func handleAccountStats(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		eventproducers.ApiRequestHandler(eventmodels.NewGetStatsRequestEventName, &eventmodels.GetStatsRequest{}, &eventmodels.GetStatsResult{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func SetupHandler(router *mux.Router) {
	router.HandleFunc("", handleAccounts)
	router.HandleFunc("/{accountName}/stats", handleAccountStats)
	router.HandleFunc("/{accountName}/strategies", handleStrategies)
}
