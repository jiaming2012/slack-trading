package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"slack-trading/src/eventconsumers"
	"slack-trading/src/eventproducers"
	"slack-trading/src/eventpubsub"
	"sync"
	"syscall"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	// Set up logger
	log.SetLevel(log.DebugLevel)

	// Set up event bus
	eventpubsub.Init()

	// Setup router
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8080"
	}

	router := mux.NewRouter()

	// Setup web server
	srv := &http.Server{
		Handler: router,
		Addr:    fmt.Sprintf(":%s", port),
	}

	// Start web server
	go func() {
		log.Infof("listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil {
			if err.Error() != "http: Server closed" {
				panic(err)
			}
		}
	}()

	// Create channel for shutdown signals.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	signal.Notify(stop, syscall.SIGTERM)

	// Start event clients
	eventproducers.NewReportClient(&wg).Start(ctx)
	eventproducers.NewSlackClient(&wg, router).Start(ctx)
	eventconsumers.NewTradeExecutorClient(&wg).Start(ctx)

	log.Info("Main: init complete")

	// Block here until program is shut down
	<-stop

	// Signal -> shut down event clients
	cancel()

	// Wait for event clients to shut down
	wg.Wait()

	fmt.Println("Main: gracefully stopped!")
}
