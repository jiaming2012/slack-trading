package optionsapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventproducers"
)

var readOptionChainRequestExector *ReadOptionChainRequestExecutor

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// eventmodels.ReadOptionChainEvent
		eventproducers.ApiRequestHandler3(&eventmodels.ReadOptionChainRequest{}, readOptionChainRequestExector, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func SetupHandler(router *mux.Router, executor *ReadOptionChainRequestExecutor) {
	readOptionChainRequestExector = executor

	router.HandleFunc("", handler)
}
