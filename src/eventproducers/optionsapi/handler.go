package optionsapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
)

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		requestExecuter := &ServeReadOptionChainRequests{}
		eventproducers.ApiRequestHandler3(eventmodels.ReadOptionChainEvent, &eventmodels.ReadOptionChainRequest{}, &eventmodels.ReadOptionChainResponse{}, requestExecuter, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func SetupHandler(router *mux.Router) {
	router.HandleFunc("", handler)
}
