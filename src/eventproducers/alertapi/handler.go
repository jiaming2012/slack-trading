package alertapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
)

func fetchAlerts(w http.ResponseWriter, r *http.Request) {
	eventproducers.ApiRequestHandler2(eventmodels.GetOptionAlertRequestEventName, &eventmodels.GetOptionAlertRequestEvent{}, &eventmodels.GetOptionAlertResponseEvent{}, w, r)
}

func createAlert(w http.ResponseWriter, r *http.Request) {
	eventproducers.ApiRequestHandler2(eventmodels.CreateOptionAlertRequestEventName, &eventmodels.CreateOptionAlertRequestEvent{}, &eventmodels.CreateOptionAlertResponseEvent{}, w, r)
}

func deleteAlert(w http.ResponseWriter, r *http.Request) {
	eventproducers.ApiRequestHandler2(eventmodels.DeleteOptionAlertRequestEventName, &eventmodels.DeleteOptionAlertRequestEvent{}, &eventmodels.DeleteOptionAlertResponseEvent{}, w, r)
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
