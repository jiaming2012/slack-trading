package strategyapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/api"
)

func handleStrategy(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		api.ApiRequestHandler2(models.GetStrategiesRequestEventName, &models.GetStrategiesRequestEvent{}, &models.GetStrategiesResponseEvent{}, w, r)
	} else if r.Method == "POST" {

	} else {
		w.WriteHeader(404)
	}
}

// todo: decrement /stratgies in favor of /accounts/:name/strategies
func SetupHandler(router *mux.Router) {
	router.HandleFunc("", handleStrategy)
}
