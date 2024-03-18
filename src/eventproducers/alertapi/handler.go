package alertapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
	"slack-trading/src/eventpubsub"
)

func fetchAlerts(w http.ResponseWriter, r *http.Request) {
	eventproducers.ApiRequestHandler(eventpubsub.GetOptionAlertRequestEvent, &eventmodels.GetOptionAlertRequestEvent{}, &eventmodels.GetOptionAlertResponseEvent{}, w, r)
}

func createAlert(w http.ResponseWriter, r *http.Request) {
	eventproducers.ApiRequestHandler(eventpubsub.CreateOptionAlertRequestEvent, &eventmodels.CreateOptionAlertRequestEvent{}, &eventmodels.CreateOptionAlertResponseEvent{}, w, r)
}

func deleteAlert(w http.ResponseWriter, r *http.Request) {
	// eventproducers.ApiRequestHandler(eventpubsub.DeleteAlertRequestEvent, &eventmodels.DeleteOptionAlertRequestEvent{}, &eventmodels.DeleteOptionAlertResponseEvent{}, w, r)
}

func handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		fetchAlerts(w, r)
	} else if r.Method == "POST" {
		createAlert(w, r)
	} else if r.Method == "DELETE" {
		deleteAlert(w, r)
	} else {
		w.WriteHeader(404)
	}
}

func SetupHandler(router *mux.Router) {
	router.HandleFunc("", handleAlerts)
}
