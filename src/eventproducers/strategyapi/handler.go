package strategyapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
)

func handleStrategy(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		eventproducers.ApiRequestHandler(eventmodels.GetStrategiesRequestEventName, &eventmodels.GetStrategiesRequestEvent{}, &eventmodels.GetStrategiesResponseEvent{}, w, r)
	} else if r.Method == "POST" {

	} else {
		w.WriteHeader(404)
	}
}

// todo: decrement /stratgies in favor of /accounts/:name/strategies
func SetupHandler(router *mux.Router) {
	router.HandleFunc("", handleStrategy)
}
