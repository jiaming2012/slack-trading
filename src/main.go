package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventpubsub"
	"slack-trading/src/handler"
	"slack-trading/src/sheets"
	"slack-trading/src/worker"
)

func main() {
	ctx := context.Background()

	// setup google sheets
	if _, _, err := sheets.Init(ctx); err != nil {
		log.Fatalf("failed to initialize google sheets: %v", err)
	}

	// setup pubsub
	eventpubsub.Init()

	// setup websocket

	// setup worker
	ch := make(chan worker.CoinbaseDTO)
	go worker.Run(ctx, ch, nil)

	// setup router
	router := mux.NewRouter()
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "3000"
	}

	router.HandleFunc("/", handler.SlackApiEventHandler)
	router.HandleFunc("/dataplane/token/balance", handler.Balance)
	router.HandleFunc("/dataplane/token/{name}", handler.Trade)
	//router.HandleFunc("/trendspider", handler.TrendSpider)   -- moved to eventmain/main.go

	srv := &http.Server{
		Handler: router,
		Addr:    fmt.Sprintf(":%s", port),
	}

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

	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("error shutting down server %s", err)
	} else {
		log.Println("Server gracefully stopped")
	}
}
