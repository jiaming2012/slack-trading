package optionsapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

var readOptionChainRequestExector *models.ReadOptionChainRequestExecutor

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// models.ReadOptionChainEvent
		// api.ApiRequestHandler3(r.Context(), &models.ReadOptionChainRequest{}, readOptionChainRequestExector, w, r)
		w.WriteHeader(404)
	} else {
		w.WriteHeader(404)
	}
}

func SetupHandler(router *mux.Router, executor *models.ReadOptionChainRequestExecutor) {
	readOptionChainRequestExector = executor

	router.HandleFunc("", handler)
}
