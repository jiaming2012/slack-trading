package router

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/backtester-api/services"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

// todo: move state to database
var (
	client            = new(eventservices.PolygonTickDataMachine)
	playgrounds       = map[uuid.UUID]models.IPlayground{}
	projectsDirectory string
	db                *gorm.DB
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

func getPlayground(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"message": "Hello, playground!",
	}

	if err := setResponse(response, w); err != nil {
		setErrorResponse("getPlayground: failed to set response", 500, err, w)
		return
	}

	w.WriteHeader(200)
}

type GetAccountResponse struct {
	Meta       *models.PlaygroundMeta      `json:"meta"`
	Balance    float64                     `json:"balance"`
	Equity     float64                     `json:"equity"`
	FreeMargin float64                     `json:"free_margin"`
	Positions  map[string]*models.Position `json:"positions"`
	Orders     []*models.BacktesterOrder   `json:"orders"`
}

type CreateAccountRequestSource struct {
	Broker     string `json:"broker"`
	AccountID  string `json:"account_id"`
	ApiKeyName string `json:"api_key_name"`
}

type CreateAccountRequest struct {
	Balance float64                    `json:"balance"`
	Source  CreateAccountRequestSource `json:"source"`
}

type CreatePlaygroundRequest struct {
	Env          string                    `json:"environment"`
	Account      CreateAccountRequest      `json:"account"`
	Clock        CreateClockRequest        `json:"clock"`
	Repositories []CreateRepositoryRequest `json:"repositories"`
	CreatedAt    time.Time                 `json:"created_at"`
}

type CreateClockRequest struct {
	StartDate string `json:"start"`
	StopDate  string `json:"stop"`
}

type RepositorySourceType string

const (
	RepositorySourcePolygon RepositorySourceType = "polygon"
	RepositorySourceCSV     RepositorySourceType = "csv"
	RepositorySourceTradier RepositorySourceType = "tradier"
)

type RepositorySource struct {
	Type        RepositorySourceType `json:"type"`
	CSVFilename *string              `json:"filename"`
}

type CreateRepositoryRequest struct {
	Symbol        string                             `json:"symbol"`
	Timespan      eventmodels.PolygonTimespanRequest `json:"timespan"`
	HistoryInDays uint32                             `json:"history_in_days"`
	Source        RepositorySource                   `json:"source"`
	Indicators    []string                           `json:"indicators"`
}

type CreateOrderRequest struct {
	Id        *uint                          `json:"id"`
	Symbol    string                         `json:"symbol"`
	Class     models.BacktesterOrderClass    `json:"class"`
	Quantity  float64                        `json:"quantity"`
	Side      models.TradierOrderSide        `json:"side"`
	OrderType models.BacktesterOrderType     `json:"type"`
	Duration  models.BacktesterOrderDuration `json:"duration"`
	Price     *float64                       `json:"price"`
	StopPrice *float64                       `json:"stop_price"`
	Tag       string                         `json:"tag"`
}

type FetchCandlesRequest struct {
	Symbol string    `json:"symbol"`
	From   time.Time `json:"from"`
	To     time.Time `json:"to"`
}

func (req *CreateOrderRequest) Validate() error {
	if err := req.Class.Validate(); err != nil {
		return fmt.Errorf("invalid class: %w", err)
	}

	if err := req.Side.Validate(); err != nil {
		return fmt.Errorf("invalid side: %w", err)
	}

	if req.Quantity <= 0 {
		return fmt.Errorf("quantity must be greater than 0")
	}

	if err := req.OrderType.Validate(); err != nil {
		return fmt.Errorf("invalid order type: %w", err)
	}

	if req.Price != nil && *req.Price <= 0 {
		return fmt.Errorf("price must be greater than 0")
	}

	if req.StopPrice != nil && *req.StopPrice <= 0 {
		return fmt.Errorf("stop price must be greater than 0")
	}

	if err := req.Duration.Validate(); err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	return nil
}

func saveTradeRecord(playgroundId uuid.UUID, orderID uint, trade *models.BacktesterTrade) error {
	record := trade.ToTradeRecord(playgroundId, orderID)
	if err := db.Create(&record).Error; err != nil {
		return fmt.Errorf("failed to save trade record: %w", err)
	}

	return nil
}

func saveOrderRecord(playgroundId uuid.UUID, order *models.BacktesterOrder) error {
	record := order.ToOrderRecord(playgroundId)
	if err := db.Create(&record).Error; err != nil {
		return fmt.Errorf("failed to save order record: %w", err)
	}

	return nil
}

func savePlaygroundSession(playground models.IPlayground) error {
	meta := playground.GetMeta()

	if err := meta.Validate(); err != nil {
		return fmt.Errorf("savePlaygroundSession: invalid playground meta: %w", err)
	}

	store := &models.PlaygroundSession{
		Id:              playground.GetId(),
		StartAt:         meta.StartAt,
		EndAt:           meta.EndAt,
		StartingBalance: meta.StartingBalance,
		Env:             string(meta.Environment),
	}

	if err := db.Create(store).Error; err != nil {
		return fmt.Errorf("failed to save playground: %w", err)
	}

	return nil
}

func makeBacktesterOrder(playground models.IPlayground, req *CreateOrderRequest, createdOn time.Time) (*models.BacktesterOrder, error) {
	var orderId uint
	if req.Id != nil {
		orderId = *req.Id
	} else {
		orderId = playground.NextOrderID()
	}

	order := models.NewBacktesterOrder(
		orderId,
		req.Class,
		createdOn,
		eventmodels.StockSymbol(req.Symbol),
		req.Side,
		req.Quantity,
		req.OrderType,
		req.Duration,
		req.Price,
		req.StopPrice,
		models.BacktesterOrderStatusPending,
		req.Tag,
	)

	changes, err := playground.PlaceOrder(order)
	if err != nil {
		return nil, fmt.Errorf("placeOrder: failed to place order: %w", err)
	}

	switch playground.(type) {
	case *models.LivePlayground:
		if err := saveOrderRecord(playground.GetId(), order); err != nil {
			return nil, fmt.Errorf("makeBacktesterOrder: failed to save order record: %w", err)
		}

		changes.AfterSave()
	}

	return order, nil
}

func handleOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(404)
		return
	}

	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		setErrorResponse("handleAccount: failed to playground id", 400, err, w)
		return
	}

	var req *CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		setErrorResponse("createOrder: failed to decode request", 400, err, w)
		return
	}

	order, err := placeOrder(id, req)
	if err != nil {
		setErrorResponse("createOrder: failed to place order", 500, err, w)
		return
	}

	if err := setResponse(order, w); err != nil {
		setErrorResponse("createOrder: failed to set response", 500, err, w)
		return
	}
}

func handleCreatePlayground(w http.ResponseWriter, r *http.Request) {
	var req CreatePlaygroundRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		setErrorResponse("createClock: failed to decode request", 400, err, w)
		return
	}

	playground, err := CreatePlayground(&req)
	if err != nil {
		webError, ok := err.(*eventmodels.WebError)
		if ok {
			setErrorResponse("createPlayground: failed to create playground", webError.StatusCode, err, w)
		} else {
			log.Warnf("failed to get status code from error: %v", err)
			setErrorResponse("createPlayground: failed to create playground", 500, err, w)
		}

		return
	}

	response := map[string]interface{}{
		"playground_id": playground.GetId(),
	}

	if err := setResponse(response, w); err != nil {
		setErrorResponse("createPlayground: failed to set response", 500, err, w)
		return
	}
}

func createClock(start, stop *eventmodels.PolygonDate) (*models.Clock, error) {
	// Load the location for New York (Eastern Time)
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("createClock: failed to load location America/New_York: %w", err)
	}

	// start at stock market open
	fromDate := time.Date(start.Year, time.Month(start.Month), start.Day, 9, 30, 0, 0, loc)

	// end at stock market close
	toDate := time.Date(stop.Year, time.Month(stop.Month), stop.Day, 16, 0, 0, 0, loc)

	// create calendar
	startDate := eventmodels.PolygonDate{
		Year:  start.Year,
		Month: start.Month,
		Day:   start.Day,
	}

	endDate := eventmodels.PolygonDate{
		Year:  stop.Year,
		Month: stop.Month,
		Day:   stop.Day,
	}

	calendar, err := services.FetchCalendarMap(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("createClock: failed to fetch calendar: %w", err)
	}

	// create clock
	clock := models.NewClock(fromDate, toDate, calendar)

	return clock, nil
}

func handleAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(404)
		return
	}

	vars := mux.Vars(r)
	playgroundID, err := uuid.Parse(vars["id"])
	if err != nil {
		setErrorResponse("handleAccount: failed to playground id", 400, err, w)
		return
	}

	fetchOrders := true
	if r.URL.Query().Get("orders") == "false" {
		fetchOrders = false
	}

	accountInfo, err := getAccountInfo(playgroundID, fetchOrders)
	if err != nil {
		setErrorResponse("handleAccount: failed to get account info", 500, err, w)
		return
	}

	if err := setResponse(accountInfo, w); err != nil {
		setErrorResponse("handleAccount: failed to set response", 500, err, w)
		return
	}
}

func handleCandles(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(404)
		return
	}

	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		setErrorResponse("handleCandles: failed to playground id", 400, err, w)
		return
	}

	// fetch from query parameters
	if err := r.ParseForm(); err != nil {
		setErrorResponse("handleCandles: failed to parse form", 400, err, w)
		return
	}

	symbol := eventmodels.StockSymbol(r.Form.Get("symbol"))
	if symbol == "" {
		setErrorResponse("handleCandles: missing symbol", 400, fmt.Errorf("missing symbol"), w)
		return
	}

	periodStr := r.Form.Get("period")
	if periodStr == "" {
		setErrorResponse("handleCandles: missing period", 400, fmt.Errorf("missing period"), w)
		return
	}

	period, err := time.ParseDuration(fmt.Sprintf("%ss", periodStr))
	if err != nil {
		setErrorResponse("handleCandles: failed to parse period", 400, err, w)
		return
	}

	fromStr := r.Form.Get("from")

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		setErrorResponse("handleCandles: failed to parse from", 400, err, w)
		return
	}

	toStr := r.Form.Get("to")
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		setErrorResponse("handleCandles: failed to parse to", 400, err, w)
		return
	}

	candles, err := fetchCandles(id, symbol, period, from, to)
	if err != nil {
		setErrorResponse("handleCandles: failed to fetch candles", 500, err, w)
		return
	}

	response := map[string]interface{}{
		"candles": candles,
	}

	if err := setResponse(response, w); err != nil {
		setErrorResponse("handleCandles: failed to set response", 500, err, w)
		return
	}
}

func handlePlayground(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		getPlayground(w, r)
	} else if r.Method == "POST" {
		handleCreatePlayground(w, r)
	} else {
		w.WriteHeader(404)
	}
}

func handleTick(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(404)
		return
	}

	// get `seconds` query parameter
	secondsStr := r.URL.Query().Get("seconds")
	if secondsStr == "" {
		setErrorResponse("handleTick: missing seconds query parameter", 400, fmt.Errorf("missing seconds query parameter"), w)
		return
	}

	duration, err := time.ParseDuration(fmt.Sprintf("%ss", secondsStr))
	if err != nil {
		setErrorResponse("handleTick: failed to parse duration", 400, err, w)
		return
	}

	// get `preview` query parameter
	previewStr := r.URL.Query().Get("preview")
	preview := false
	if previewStr != "" {
		var err error
		preview, err = utils.ParseBool(previewStr)
		if err != nil {
			setErrorResponse("handleTick: failed to parse preview", 400, err, w)
			return
		}
	}

	// get playground id
	vars := mux.Vars(r)
	playgroundId, err := uuid.Parse(vars["id"])
	if err != nil {
		setErrorResponse("handleTick: failed to playground id", 400, err, w)
		return
	}

	// tick
	stateChange, err := nextTick(playgroundId, duration, preview)
	if err != nil {
		setErrorResponse("handleTick: failed to tick", 500, err, w)
		return
	}

	if err := setResponse(stateChange, w); err != nil {
		setErrorResponse("handleTick: failed to set response", 500, err, w)
		return
	}
}

func findOrder(id uint) (models.IPlayground, *models.BacktesterOrder, bool) {
	for _, playground := range playgrounds {
		orders := playground.GetOrders()
		for _, order := range orders {
			if order.ID == id {
				return playground, order, true
			}
		}
	}

	return nil, nil, false
}

func handleLiveOrders(ctx context.Context, queue *eventmodels.FIFOQueue[*eventmodels.TradierOrderUpdateEvent]) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			for {
				event, ok := queue.Dequeue()
				if !ok {
					break
				}

				if event.CreateOrder != nil {
					playground, order, found := findOrder(event.CreateOrder.Order.ID)

					if found {
						if event.CreateOrder.Order.Status == string(models.BacktesterOrderStatusFilled) {
							performChecks := false
							positions := playground.GetPositions()
							fillOrderPriceMap := map[*models.BacktesterOrder]models.OrderFillEntry{
								order: {
									Time:  event.CreateOrder.Order.CreateDate,
									Price: event.CreateOrder.Order.AvgFillPrice,
								},
							}

							new_trades, invalid_orders, err := playground.CommitPendingOrders(positions, fillOrderPriceMap, performChecks)
							if err != nil {
								log.Fatalf("handleLiveOrders: failed to commit pending orders: %v", err)
							}

							for _, trade := range new_trades {
								if err := saveTradeRecord(playground.GetId(), order.ID, trade); err != nil {
									log.Fatalf("handleLiveOrders: failed to save trade record: %v", err)
								}
							}

							if len(invalid_orders) > 0 {
								log.Fatalf("handleLiveOrders: invalid orders found: %v", invalid_orders)
							}

							for _, trade := range new_trades {
								log.Infof("handleLiveOrders: opened trade: %v", trade)
							}

						} else if event.CreateOrder.Order.Status == string(models.BacktesterOrderStatusRejected) {
							reason := "rejected by broker"
							if event.CreateOrder.Order.ReasonDescription != nil {
								reason = *event.CreateOrder.Order.ReasonDescription
							}

							if err := playground.RejectOrder(order, reason); err != nil {
								log.Errorf("handleLiveOrders: failed to reject order: %v", err)
							}
						} else {
							log.Fatalf("handleLiveOrders: unknown order status: %v", event.CreateOrder.Order.Status)
						}
					}
				} else if event.ModifyOrder != nil {

				} else if event.DeleteOrder != nil {
					// pass
				} else {
					log.Warnf("handleLiveOrders: unknown event type: %v", event)
				}
			}
		}
	}
}

func SetupHandler(ctx context.Context, router *mux.Router, projectsDir string, apiKey string, ordersUpdateQueue *eventmodels.FIFOQueue[*eventmodels.TradierOrderUpdateEvent], database *gorm.DB) {
	client = eventservices.NewPolygonTickDataMachine(apiKey)
	db = database
	projectsDirectory = projectsDir

	router.HandleFunc("", handlePlayground)
	router.HandleFunc("/{id}/account", handleAccount)
	router.HandleFunc("/{id}/order", handleOrder)
	router.HandleFunc("/{id}/tick", handleTick)
	router.HandleFunc("/{id}/candles", handleCandles)

	go handleLiveOrders(ctx, ordersUpdateQueue)
}
