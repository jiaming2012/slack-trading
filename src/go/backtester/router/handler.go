package router

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	backtester_models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/backtester/services"
	"github.com/jiaming2012/slack-trading/src/go/data"
	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/marketdata"
)

var (
	client            = new(marketdata.PolygonTickDataMachine)
	projectDirectory string
	database          backtester_models.IDatabaseService
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
func handleLiveOrders(ctx context.Context, orderUpdateQueue *models.FIFOQueue[*backtester_models.TradierOrderUpdateEvent], database backtester_models.IDatabaseService) {
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

func SetupHandler(ctx context.Context, router *mux.Router, projectDir string, apiKey string, ordersUpdateQueue *models.FIFOQueue[*backtester_models.TradierOrderUpdateEvent], dbService *data.DatabaseService, brokerMap map[backtester_models.CreateAccountRequestSource]backtester_models.IBroker, calendar *models.MarketCalendar) error {
	client = marketdata.NewPolygonClient(apiKey)
	projectDirectory = projectDir

	if err := loadData(dbService, brokerMap, calendar); err != nil {
		return fmt.Errorf("SetupHandler: failed to load data: %w", err)
	}

	handleLiveOrders(ctx, ordersUpdateQueue, dbService)

	return nil
}
