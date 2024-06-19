package accountapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventproducers"
)

func handleAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		eventproducers.ApiRequestHandler2(eventmodels.GetAccountsRequestEventName, &eventmodels.GetAccountsRequestEvent{}, &eventmodels.GetAccountsResponseEvent{}, w, r)
	} else if r.Method == "POST" {
		eventproducers.ApiRequestHandler2(eventmodels.CreateAccountRequestEventName, &eventmodels.CreateAccountRequestEventV1{}, &eventmodels.CreateAccountResponseEvent{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func handleStrategies(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.WriteHeader(404)
	} else if r.Method == "POST" {
		eventproducers.ApiRequestHandler2(eventmodels.CreateAccountStrategyRequestEventName, &eventmodels.CreateAccountStrategyRequestEvent{}, &eventmodels.CreateAccountStrategyResponseEvent{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func handleAccountStats(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		eventproducers.ApiRequestHandler2(eventmodels.NewGetStatsRequestEventName, &eventmodels.GetStatsRequest{}, &eventmodels.GetStatsResult{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func SetupHandler(router *mux.Router) {
	router.HandleFunc("", handleAccounts)
	router.HandleFunc("/{accountName}/stats", handleAccountStats)
	router.HandleFunc("/{accountName}/strategies", handleStrategies)
}
