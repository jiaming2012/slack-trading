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
	"syscall"
	"time"
)

func main() {
	router := mux.NewRouter()
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "3000"
	}

	router.HandleFunc("/", handler.SlackApiEventHandler)
	router.HandleFunc("/dataplane/token/{name}", handler.Trade)

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

	//// create api context
	//ctx := context.Background()
	//
	//// authenticate and setup service
	//srv, err := sheets.Setup(ctx)
	//if err != nil {
	//	log.Fatalf("setup failed: %v", err)
	//}
	//
	//trade := &models.Trade{
	//	Time:   time.Now(),
	//	Symbol: "ETHUSD",
	//	Volume: -1.2,
	//	RequestedPrice:  2340.60,
	//}
	//
	//err = sheets.AppendTrade(ctx, srv, trade)
	//if err != nil {
	//	log.Fatal(err)
	//}
	////appendRow(ctx, srv, spreadsheetId, "Sheet1")
	////updateRow(ctx, srv, spreadsheetId, "Sheet2")
	////rows, err := fetchRows(ctx, srv, spreadsheetId, "Sheet1", "A3:C7")
	////if err != nil {
	////	log.Fatal(err)
	////}
	//
	//trades, err := sheets.FetchTrades(ctx, srv, "ETHUSD")
	//if err != nil {
	//	log.Fatal(err)
	//}
	//
	//fmt.Println(trades)
}
