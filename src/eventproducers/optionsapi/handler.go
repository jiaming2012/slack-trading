package optionsapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
)

var readOptionChainRequestExector *ReadOptionChainRequestExecutor

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		eventproducers.ApiRequestHandler3(eventmodels.ReadOptionChainEvent, &eventmodels.ReadOptionChainRequest{}, &eventmodels.ReadOptionChainResponse{}, readOptionChainRequestExector, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func SetupHandler(router *mux.Router, executor *ReadOptionChainRequestExecutor) {
	readOptionChainRequestExector = executor

	router.HandleFunc("", handler)
}
