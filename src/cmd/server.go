package server

import (
	"github.com/gorilla/mux"




































































































	"github.com/jiaming2012/slack-trading/src/handler"
)

func Setup() *mux.Router {
	router := mux.NewRouter()

	router.HandleFunc("/", handler.SlackApiEventHandler)
	router.HandleFunc("/dataplane/token/{name}", handler.Trade)

	return router
}
