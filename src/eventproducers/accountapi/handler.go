package accountapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
	pubsub "slack-trading/src/eventpubsub"
)

func getAccountStats(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		req := &eventmodels.GetStatsRequest{}

		// todo: this can also be removed (once ApiRequestHandler is used)
		if err := req.Validate(r); err != nil {
			if respErr := eventproducers.SetErrorResponse("validation", 400, err, w); respErr != nil {
				log.Errorf("getAccountStats: failed to set error response: %v", respErr)
			}
			return
		}

		// todo: refactor this to use the eventproducers.ApiRequestHandler
		req.RequestID = uuid.New()
		resultCh, errCh := eventmodels.RegisterResultCallback(req.RequestID)

		pubsub.Publish("getAccountStats", pubsub.NewGetStatsRequest, req)

		select {
		case result := <-resultCh:
			res, ok := result.(*eventmodels.GetStatsResult)
			if !ok {
				log.Errorf("getAccountStats: failed to read GetStatsResult")
				w.WriteHeader(500)
				return
			}

			if err := eventproducers.SetResponse(res, w); err != nil {
				log.Errorf("getAccountStats: failed to set response: %v", err)
				w.WriteHeader(500)
				return
			}
		case err := <-errCh:
			if respErr := eventproducers.SetErrorResponse("req", 400, err, w); respErr != nil {
				log.Errorf("getAccountStats: failed to set error response: %v", respErr)
				w.WriteHeader(500)
				return
			}
		}
	} else {
		w.WriteHeader(404)
	}
}

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
		eventproducers.ApiRequestHandler(pubsub.CreateStrategyRequestEvent, &eventmodels.AccountsStrategiesPostRequest{}, &eventmodels.AccountsStrategiesPostRequest{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func SetupHandler(router *mux.Router) {
	router.HandleFunc("", handleAccounts)
	router.HandleFunc("/{accountName}/stats", getAccountStats)
	router.HandleFunc("/{accountName}/strategies", handleStrategies)
}
