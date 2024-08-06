package optionsapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

var readOptionChainRequestExector *eventmodels.ReadOptionChainRequestExecutor

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// eventmodels.ReadOptionChainEvent
		// eventproducers.ApiRequestHandler3(r.Context(), &eventmodels.ReadOptionChainRequest{}, readOptionChainRequestExector, w, r)
		w.WriteHeader(404)
	} else {
		w.WriteHeader(404)
	}
}

func SetupHandler(router *mux.Router, executor *eventmodels.ReadOptionChainRequestExecutor) {
	readOptionChainRequestExector = executor

	router.HandleFunc("", handler)
}
