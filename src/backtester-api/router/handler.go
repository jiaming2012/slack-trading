package router

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/backtester-api/services"
	"github.com/jiaming2012/slack-trading/src/data"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
)

var (
	client            = new(eventservices.PolygonTickDataMachine)
	projectsDirectory string
	database          models.IDatabaseService
)

type errorResponse struct {
	Type string `json:"type"`
	Msg  string `json:"message"`
}

func NewErrorResponse(errType string, message string) *errorResponse {
	return &errorResponse{
		Type: errType,
		Msg:  message,
	}
}

func setResponse(response interface{}, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		return fmt.Errorf("SetResponse: encode: %w", err)
	}

	return nil
}

func setErrorResponse(errType string, statusCode int, err error, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := NewErrorResponse(errType, err.Error())
	if encodeErr := json.NewEncoder(w).Encode(resp); encodeErr != nil {
		return encodeErr
	}

	return nil
}

type FetchCandlesRequest struct {
	Symbol string    `json:"symbol"`
	From   time.Time `json:"from"`
	To     time.Time `json:"to"`
}

// handles live order from broker
func handleLiveOrders(ctx context.Context, orderUpdateQueue *eventmodels.FIFOQueue[*models.TradierOrderUpdateEvent], database models.IDatabaseService) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Debug("handleLiveOrders: context done")
				return
			default:
				hasUpdates, err := services.DrainTradierOrderQueue(orderUpdateQueue, database)
				if err != nil {
					log.Errorf("handleLiveOrders: failed to drain order queue: %v", err)
					continue
				}

				if !hasUpdates {
					log.Tracef("handleLiveOrders: no order update events. Sleeping for 1 second(s) ...")
					time.Sleep(1 * time.Second)
					log.Tracef("handleLiveOrders: waking up")
				}
			}
		}
	}()
}

func SetupHandler(ctx context.Context, router *mux.Router, projectsDir string, apiKey string, ordersUpdateQueue *eventmodels.FIFOQueue[*models.TradierOrderUpdateEvent], dbService *data.DatabaseService, brokerMap map[models.CreateAccountRequestSource]models.IBroker, calendar *eventmodels.MarketCalendar) error {
	client = eventservices.NewPolygonClient(apiKey)
	projectsDirectory = projectsDir

	if err := loadData(dbService, brokerMap, calendar); err != nil {
		return fmt.Errorf("SetupHandler: failed to load data: %w", err)
	}

	handleLiveOrders(ctx, ordersUpdateQueue, dbService)

	return nil
}
