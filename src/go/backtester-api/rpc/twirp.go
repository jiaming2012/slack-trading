package rpc

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	backtester_router "github.com/jiaming2012/slack-trading/src/go/backtester-api/router"
	"github.com/jiaming2012/slack-trading/src/go/data"
	"github.com/jiaming2012/slack-trading/src/go/eventservices"
	"github.com/jiaming2012/slack-trading/src/go/playground"
)

// panicRecoveryMiddleware catches panics in HTTP handlers and logs them via logrus
// instead of letting them go to stderr where they're invisible.
func panicRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Errorf("panic in %s %s: %v", r.Method, r.URL.Path, rec)
				http.Error(w, fmt.Sprintf("internal error: %v", rec), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func SetupTwirpServer(optionsClient *eventservices.PolygonOptionsClient, dbService *data.DatabaseService) {
	server := backtester_router.NewServer(optionsClient, dbService)
	twirpHandler := playground.NewPlaygroundServiceServer(server)
	port := 5051

	mux := http.NewServeMux()
	mux.Handle(twirpHandler.PathPrefix(), otelhttp.NewHandler(panicRecoveryMiddleware(twirpHandler), "twirp"))

	log.Infof("Twirp server listening on :%d", port)
	log.Infof("Path prefix: %v", twirpHandler.PathPrefix())

	http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}
