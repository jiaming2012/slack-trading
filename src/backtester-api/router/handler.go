package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
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

type orderCache struct {
	container map[uint]models.ExecutionFillRequest
	mutex     *sync.Mutex
}

func (c *orderCache) Add(order *eventmodels.TradierOrder, entry models.ExecutionFillRequest) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.container[order.ID] = entry
}

func (c *orderCache) Get(order *eventmodels.TradierOrder) (models.ExecutionFillRequest, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry, ok := c.container[order.ID]
	return entry, ok
}

func (c *orderCache) GetMap() (container map[uint]models.ExecutionFillRequest, unlockFn func()) {
	c.mutex.Lock()
	container = c.container

	unlockFn = func() {
		c.mutex.Unlock()
	}

	return
}

func (c *orderCache) Remove(orderID uint, getMutex bool) {
	if getMutex {
		c.mutex.Lock()
		defer c.mutex.Unlock()
	}

	delete(c.container, orderID)
}

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

func handleLiveOrders(ctx context.Context, orderUpdateQueue *eventmodels.FIFOQueue[*models.TradierOrderUpdateEvent]) {
	cache := &orderCache{
		container: make(map[uint]models.ExecutionFillRequest),
		mutex:     &sync.Mutex{},
	}

	commitPendingOrders := func() {
		orderCache, unlockFn := cache.GetMap()

		defer unlockFn()

		for tradierOrder, orderFillEntry := range orderCache {
			playground, order, err := database.FindOrder(orderFillEntry.PlaygroundId, tradierOrder)
			if err != nil {
				log.Errorf("handleLiveOrders: failed to find orders: %v", err)
				continue
			}

			positions, err := playground.GetPositions()
			if err != nil {
				log.Errorf("handleLiveOrders: failed to get positions: %v", err)
				continue
			}

			performChecks := false

			trade, err := playground.FillOrder(order, performChecks, orderFillEntry, positions)
			if err != nil {
				log.Errorf("handleLiveOrders: failed to commit pending orders: %v", err)

				if errors.Is(err, models.ErrTradingNotAllowed) {
					log.Debugf("handleLiveOrders: removing order from cache: %v", tradierOrder)
					cache.Remove(tradierOrder, false)
				}

				continue
			}

			// Resave the order to update the status and close_id
			balance := playground.GetBalance()
			if err := database.SaveOrderRecord(playground.GetId(), order, &balance, playground.GetLiveAccountType()); err != nil {
				if errors.Is(err, models.ErrDbOrderIsNotOpenOrPending) {
					log.Warnf("handleLiveOrders: order is not open or pending: %v", err)

					cache.Remove(tradierOrder, false)
					continue
				}

				log.Fatalf("handleLiveOrders: failed to save order record: %v", err)
			}

			if livePlayground, ok := playground.(*models.LivePlayground); ok {
				livePlayground.GetNewTradeQueue().Enqueue(trade)
			} else {
				log.Errorf("handleLiveOrders: playground is not live: %v", playground)
			}

			log.Infof("handleLiveOrders: opened trade: %v", trade)

			cache.Remove(tradierOrder, false)
		}
	}

	// commit pending orders from cache
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				commitPendingOrders()
				time.Sleep(10 * time.Second)
			}
		}
	}()

	// handles order from broker
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Debug("handleLiveOrders: context done")
				return
			default:
				event, ok := orderUpdateQueue.Dequeue()
				if !ok {
					time.Sleep(8 * time.Second)
					log.Tracef("handleLiveOrders: no order update events ... waking up")
					continue
				}

				if event.CreateOrder != nil {
					if event.CreateOrder.Order.Status == string(models.BacktesterOrderStatusFilled) {
						cache.Add(event.CreateOrder.Order, models.ExecutionFillRequest{
							PlaygroundId: event.CreateOrder.OrderRecord.PlaygroundID,
							Time:         event.CreateOrder.Order.CreateDate,
							Price:        event.CreateOrder.Order.AvgFillPrice,
							Quantity:     event.CreateOrder.Order.GetLastFillQuantity(),
						})

						log.Debugf("handleLiveOrders: order filled: %v", event.CreateOrder.Order)
					} else if event.CreateOrder.Order.Status == string(models.BacktesterOrderStatusPending) {
						log.Debugf("handleLiveOrders: order pending: %v", event.CreateOrder.Order)
					} else {
						log.Fatalf("handleLiveOrders: unknown order status: %v", event.CreateOrder.Order.Status)
					}

				} else if event.ModifyOrder != nil {
					if event.ModifyOrder.Field == "status" {
						playground, order, err := database.FindOrder(event.ModifyOrder.PlaygroundId, event.ModifyOrder.OrderID)

						if err == nil {
							reason, ok := event.ModifyOrder.New.(string)
							if !ok {
								log.Errorf("handleLiveOrders: failed to convert reason to string: %v", event.ModifyOrder.New)
								continue
							}

							if err := playground.RejectOrder(order, reason); err != nil {
								log.Errorf("handleLiveOrders: failed to reject order: %v", err)
							}

							if err := database.SaveOrderRecord(playground.GetId(), order, nil, playground.GetLiveAccountType()); err != nil {
								log.Fatalf("handleLiveOrders: failed to save order record: %v", err)
							}
						} else {
							log.Warnf("handleLiveOrders: order not found: %v", event.ModifyOrder.OrderID)
						}
					}
				} else if event.DeleteOrder != nil {
					// pass
				} else {
					log.Warnf("handleLiveOrders: unknown event type: %v", event)
				}
			}
		}
	}()
}

func SetupHandler(ctx context.Context, router *mux.Router, projectsDir string, apiKey string, ordersUpdateQueue *eventmodels.FIFOQueue[*models.TradierOrderUpdateEvent], apiService *services.BacktesterApiService, dbService *data.DatabaseService) error {
	client = eventservices.NewPolygonTickDataMachine(apiKey)
	projectsDirectory = projectsDir

	if err := loadData(apiService, dbService); err != nil {
		return fmt.Errorf("SetupHandler: failed to load data: %w", err)
	}

	handleLiveOrders(ctx, ordersUpdateQueue)

	return nil
}
