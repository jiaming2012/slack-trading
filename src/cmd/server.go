package server

import (
	"github.com/gorilla/mux"
	"slack-trading/src/handler"
)

func Setup() *mux.Router {
	router := mux.NewRouter()

	router.HandleFunc("/", handler.SlackApiEventHandler)
	router.HandleFunc("/dataplane/token/{name}", handler.Trade)

	return router
}
