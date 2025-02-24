package rpc

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	backtester_router "github.com/jiaming2012/slack-trading/src/backtester-api/router"
	"github.com/jiaming2012/slack-trading/src/playground"
)

func SetupTwirpServer() {
	server := backtester_router.NewServer()
	twirpHandler := playground.NewPlaygroundServiceServer(server)
	port := 5051

	mux := http.NewServeMux()
	mux.Handle(twirpHandler.PathPrefix(), twirpHandler)

	log.Infof("Twirp server listening on :%d", port)
	log.Infof("Path prefix: %v", twirpHandler.PathPrefix())

	http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}
