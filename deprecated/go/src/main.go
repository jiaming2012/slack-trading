package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	pb "github.com/jiaming2012/slack-trading/src/backtester-api/playground"
	"github.com/jiaming2012/slack-trading/src/eventpubsub"
	"github.com/jiaming2012/slack-trading/src/handler"
	"github.com/jiaming2012/slack-trading/src/sheets"
	"github.com/jiaming2012/slack-trading/src/worker"
)

type GrpcServer struct {
	pb.UnimplementedPlaygroundServiceServer
}

func (s *GrpcServer) CreatePlayground(ctx context.Context, in *pb.CreatePolygonPlaygroundRequest) (*pb.CreatePlaygroundResponse, error) {
	return &pb.CreatePlaygroundResponse{}, nil
}

func main() {
	ctx := context.Background()

	// setup google sheets
	if _, _, err := sheets.NewClientFromEnv(ctx); err != nil {
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

	// start the http server
	srv := &http.Server{
		Handler: router,
		Addr:    fmt.Sprintf(":%s", port),
	}

	go func() {
		log.Infof("listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil {
			if err.Error() != "http: Server closed" {
				log.Fatalf("http: failed to listen and serve: %v", err)
			}
		}
	}()

	// start the grpc server
	go func() {
		grpcServer := grpc.NewServer()
		pb.RegisterPlaygroundServiceServer(grpcServer, &GrpcServer{})
		port := 50051

		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}

		log.Infof("listening on :%d", port)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("grpc: failed to serve: %v", err)
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
