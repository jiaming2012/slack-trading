package accountapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/api"
)

func handleAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		api.ApiRequestHandler2(models.GetAccountsRequestEventName, &models.GetAccountsRequestEvent{}, &models.GetAccountsResponseEvent{}, w, r)
	} else if r.Method == "POST" {
		api.ApiRequestHandler2(models.CreateAccountRequestEventName, &models.CreateAccountRequestEventV1{}, &models.CreateAccountResponseEvent{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func handleStrategies(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.WriteHeader(404)
	} else if r.Method == "POST" {
		api.ApiRequestHandler2(models.CreateAccountStrategyRequestEventName, &models.CreateAccountStrategyRequestEvent{}, &models.CreateAccountStrategyResponseEvent{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func handleAccountStats(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		api.ApiRequestHandler2(models.NewGetStatsRequestEventName, &models.GetStatsRequest{}, &models.GetStatsResult{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func SetupHandler(router *mux.Router) {
	router.HandleFunc("", handleAccounts)
	router.HandleFunc("/{accountName}/stats", handleAccountStats)
	router.HandleFunc("/{accountName}/strategies", handleStrategies)
}
