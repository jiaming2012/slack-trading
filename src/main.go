package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"slack-trading/src/handler"
	"slack-trading/src/sheets"
	"syscall"
	"time"
)

func main() {
	ctx := context.Background()

	// setup google sheets
	if err := sheets.Init(ctx); err != nil {
		panic(fmt.Errorf("failed to initialize google sheets: %v", err))
	}

	// setup router
	router := mux.NewRouter()
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "3000"
	}

	router.HandleFunc("/", handler.SlackApiEventHandler)
	router.HandleFunc("/dataplane/token/{name}", handler.Trade)
	router.HandleFunc("/dataplane/token/balance", handler.Balance)

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
