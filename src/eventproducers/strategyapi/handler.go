package strategyapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
	pubsub "slack-trading/src/eventpubsub"
)

func handleStrategy(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		eventproducers.ApiRequestHandler(pubsub.GetStrategiesRequestEvent, &eventmodels.GetStrategiesRequestEvent{}, &eventmodels.GetStrategiesResponseEvent{}, w, r)
	} else if r.Method == "POST" {

	} else {
		w.WriteHeader(404)
	}
}

// todo: decrement /stratgies in favor of /accounts/:name/strategies
func SetupHandler(router *mux.Router) {
	router.HandleFunc("", handleStrategy)
}
