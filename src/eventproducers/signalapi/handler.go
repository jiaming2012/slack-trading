package signalapi

import (
	"github.com/gorilla/mux"
	"net/http"
	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
	pubsub "slack-trading/src/eventpubsub"
	"slack-trading/src/models"
)

func signalsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		eventproducers.SignalRequestHandler(pubsub.NewSignalsRequest, &models.SignalRequest{}, &eventmodels.NewSignalResult{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func SetupHandler(router *mux.Router) {
	router.HandleFunc("", signalsHandler)
}
