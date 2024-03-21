package signalapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
	pubsub "slack-trading/src/eventpubsub"
)

func signalsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		eventproducers.ApiRequestHandler(pubsub.CreateSignalRequestEvent, &eventmodels.CreateSignalRequest{}, &eventmodels.CreateSignalResultEvent{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func SetupHandler(router *mux.Router) {
	router.HandleFunc("", signalsHandler)
}
