package alertapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/eventproducers"
)

func fetchAlerts(w http.ResponseWriter, r *http.Request) {
	eventproducers.ApiRequestHandler2(models.GetOptionAlertRequestEventName, &models.GetOptionAlertRequestEvent{}, &models.GetOptionAlertResponseEvent{}, w, r)
}

func createAlert(w http.ResponseWriter, r *http.Request) {
	eventproducers.ApiRequestHandler2(models.CreateOptionAlertRequestEventName, &models.CreateOptionAlertRequestEvent{}, &models.CreateOptionAlertResponseEvent{}, w, r)
}

func deleteAlert(w http.ResponseWriter, r *http.Request) {
	eventproducers.ApiRequestHandler2(models.DeleteOptionAlertRequestEventName, &models.DeleteOptionAlertRequestEvent{}, &models.DeleteOptionAlertResponseEvent{}, w, r)
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
