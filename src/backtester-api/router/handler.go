package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
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

// todo: add a mutex playground level
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

type GetAccountResponse struct {
	Meta       *models.PlaygroundMeta      `json:"meta"`
	Balance    float64                     `json:"balance"`
	Equity     float64                     `json:"equity"`
	FreeMargin float64                     `json:"free_margin"`
	Positions  map[string]*models.Position `json:"positions"`
	Orders     []*models.BacktesterOrder   `json:"orders"`
}

type CreateAccountRequestSource struct {
	Broker      string                 `json:"broker"`
	AccountID   string                 `json:"account_id"`
	AccountType models.LiveAccountType `json:"account_type"`
}

type CreateAccountRequest struct {
	Balance float64                     `json:"balance"`
	Source  *CreateAccountRequestSource `json:"source"`
}

type CreatePlaygroundRequest struct {
	ID                *uuid.UUID                            `json:"playground_id"`
	ClientID          *string                               `json:"client_id"`
	Env               string                                `json:"environment"`
	Account           CreateAccountRequest                  `json:"account"`
	InitialBalance    float64                               `json:"starting_balance"`
	Clock             CreateClockRequest                    `json:"clock"`
	Repositories      []eventmodels.CreateRepositoryRequest `json:"repositories"`
	BackfillOrders    []*models.BacktesterOrder             `json:"orders"`
	EquityPlotRecords []*eventmodels.EquityPlot             `json:"equity_plot_records"`
	CreatedAt         time.Time                             `json:"created_at"`
	SaveToDB          bool                                  `json:"-"`
}

type CreateClockRequest struct {
	StartDate string `json:"start"`
	StopDate  string `json:"stop"`
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

func fetchOrderIdFromDbByExternalOrderId(playgroundId uuid.UUID, externalOrderID uint) (uint, bool) {
	var orderRecord models.OrderRecord

	if result := db.First(&orderRecord, "playground_id = ? AND external_id = ?", playgroundId, externalOrderID); result.Error != nil {
		return 0, false
	}

	return orderRecord.ID, true
}

func saveOrderRecordsTx(tx *gorm.DB, playgroundId uuid.UUID, orders []*models.BacktesterOrder, liveAccountType *models.LiveAccountType) ([]*models.OrderRecord, error) {
	var allOrderRecords []*models.OrderRecord
	var updateOrderRequests []*models.UpdateOrderRecordRequest

	for _, order := range orders {
		var err error

		oRec, updateOrderReq, err := order.ToOrderRecord(tx, playgroundId, liveAccountType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert order to order record: %w", err)
		}

		updateOrderRequests = append(updateOrderRequests, updateOrderReq...)

		oID, found := fetchOrderIdFromDbByExternalOrderId(playgroundId, oRec.ExternalOrderID)
		if found {
			oRec.ID = oID
		}

		if err = tx.Save(&oRec).Error; err != nil {
			return nil, fmt.Errorf("failed to save order records: %w", err)
		}

		allOrderRecords = append(allOrderRecords, oRec)
	}

	// wait for all orders to be saved before updating the closes
	for _, updateReq := range updateOrderRequests {
		if updateReq == nil {
			continue
		}

		switch updateReq.Field {
		case "closes":
			var closes []*models.OrderRecord
			for _, order := range updateReq.Closes {
				orderRec, err := order.FetchOrderRecordFromDB(tx, *updateReq.PlaygroundId)
				if err != nil {
					return nil, fmt.Errorf("updateOrderRequests: failed to fetch close order record from db: %w", err)
				}

				closes = append(closes, orderRec)
			}

			updateReq.OrderRecord.Closes = closes
			if err := tx.Save(updateReq.OrderRecord).Error; err != nil {
				return nil, fmt.Errorf("updateOrderRequests: failed to update order record (closes): %w", err)
			}

		case "closed_by":
			updateReq.OrderRecord.ClosedBy = updateReq.ClosedBy
			if err := tx.Save(updateReq.OrderRecord).Error; err != nil {
				return nil, fmt.Errorf("updateOrderRequests: failed to update order record (close_by): %w", err)
			}

		default:
			return nil, fmt.Errorf("updateOrderRequests: field %s not implemented", updateReq.Field)
		}
	}

	return allOrderRecords, nil
}

func saveBalance(tx *gorm.DB, playgroundId uuid.UUID, balance float64) error {
	if result := tx.Model(&models.PlaygroundSession{}).Where("id = ?", playgroundId).Update("balance", balance); result.Error != nil {
		return fmt.Errorf("saveBalance: failed to save balance: %w", result.Error)
	}

	return nil
}

func saveOrderRecord(playgroundId uuid.UUID, order *models.BacktesterOrder, newBalance *float64, liveAccountType *models.LiveAccountType) error {
	err := db.Transaction(func(tx *gorm.DB) error {
		if _, err := saveOrderRecordsTx(tx, playgroundId, []*models.BacktesterOrder{order}, liveAccountType); err != nil {
			return fmt.Errorf("saveOrderRecord: failed to save order records: %w", err)
		}

		if newBalance != nil {
			if err := saveBalance(tx, playgroundId, *newBalance); err != nil {
				return fmt.Errorf("saveOrderRecord: failed to save balance: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("saveOrderRecord: save order record transaction failed: %w", err)
	}

	return nil
}

type closeByRequest struct {
	OrderID     uint
	TradeRecord *models.TradeRecord
}

func findOrderRec(id uint, orders []*models.OrderRecord) (*models.OrderRecord, error) {
	for _, order := range orders {
		if order.ID == id {
			return order, nil
		}
	}

	return nil, fmt.Errorf("findOrderRec: failed to find order record: %d", id)
}

func deletePlaygroundSession(playground models.IPlayground) error {
	session := &models.PlaygroundSession{
		ID: playground.GetId(),
	}

	if err := db.Delete(&session).Error; err != nil {
		return fmt.Errorf("deletePlayground: failed to delete playground: %w", err)
	}

	return nil
}

func savePlayground(playground models.IPlayground) error {
	err := db.Transaction(func(tx *gorm.DB) error {
		var txErr error

		if txErr = savePlaygroundSessionTx(tx, playground); txErr != nil {
			return fmt.Errorf("failed to save playground session: %w", txErr)
		}

		playgroundId := playground.GetId()
		meta := playground.GetMeta()
		if meta == nil {
			return errors.New("savePlayground: missing playground meta")
		}

		if _, txErr = saveOrderRecordsTx(tx, playgroundId, playground.GetOrders(), meta.LiveAccountType); txErr != nil {
			return fmt.Errorf("failed to save order records: %w", txErr)
		}

		if txErr = saveEquityPlotRecords(tx, playgroundId, playground.GetEquityPlot()); txErr != nil {
			return fmt.Errorf("failed to save equity plot records: %w", txErr)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("savePlayground: failed to save playground: %w", err)
	}

	return nil
}

func saveEquityPlotRecords(tx *gorm.DB, playgroundId uuid.UUID, records []*eventmodels.EquityPlot) error {
	var equityPlotRecords []*models.EquityPlotRecord

	for _, record := range records {
		equityPlotRecords = append(equityPlotRecords, &models.EquityPlotRecord{
			PlaygroundID: playgroundId,
			Timestamp:    record.Timestamp,
			Equity:       record.Value,
		})
	}

	if err := tx.CreateInBatches(equityPlotRecords, 100).Error; err != nil {
		return fmt.Errorf("saveEquityPlotRecords: failed to save equity plot records: %w", err)
	}

	return nil
}

func saveEquityPlotRecord(playgroundId uuid.UUID, timestamp time.Time, equity float64) error {
	rec := &models.EquityPlotRecord{
		PlaygroundID: playgroundId,
		Timestamp:    timestamp,
		Equity:       equity,
	}

	if err := db.Create(rec).Error; err != nil {
		return fmt.Errorf("saveEquityPlotRecord: failed to save equity plot record: %w", err)
	}

	return nil
}

func savePlaygroundSessionTx(tx *gorm.DB, playground models.IPlayground) error {
	meta := playground.GetMeta()

	if err := meta.Validate(); err != nil {
		return fmt.Errorf("savePlaygroundSession: invalid playground meta: %w", err)
	}

	repos := playground.GetRepositories()
	var repoDTOs []models.CandleRepositoryDTO
	for _, repo := range repos {
		repoDTOs = append(repoDTOs, repo.ToDTO())
	}

	store := &models.PlaygroundSession{
		ID:              playground.GetId(),
		ClientID:        playground.GetClientId(),
		CurrentTime:     playground.GetCurrentTime(),
		StartAt:         meta.StartAt,
		EndAt:           meta.EndAt,
		Balance:         playground.GetBalance(),
		StartingBalance: meta.InitialBalance,
		Repositories:    models.CandleRepositoryRecord(repoDTOs),
		Env:             string(meta.Environment),
	}

	if meta.Environment == models.PlaygroundEnvironmentLive {
		store.Broker = &meta.SourceBroker
		store.AccountID = &meta.SourceAccountId

		liveAccountType := string(*meta.LiveAccountType)
		store.LiveAccountType = &liveAccountType
	}

	if err := tx.Create(store).Error; err != nil {
		return fmt.Errorf("failed to save playground: %w", err)
	}

	return nil
}

func savePlaygroundSession(playground models.IPlayground) error {
	return savePlaygroundSessionTx(db, playground)
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
		if err := saveOrderRecord(playground.GetId(), order, nil, playground.GetLiveAccountType()); err != nil {
			return nil, fmt.Errorf("makeBacktesterOrder: failed to save order record: %w", err)
		}
	}

	changes.Commit()

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

func loadPlaygrounds() error {
	var playgroundsSlice []models.PlaygroundSession
	if err := db.Preload("Orders").Preload("Orders.Trades").Preload("Orders.Closes").Preload("Orders.ClosedBy").Preload("Orders.Closes.ClosedBy").Preload("EquityPlotRecords").Find(&playgroundsSlice).Error; err != nil {
		return fmt.Errorf("loadPlaygrounds: failed to load playgrounds: %w", err)
	}

	for _, p := range playgroundsSlice {
		orders := make([]*models.BacktesterOrder, len(p.Orders))

		pIDStr := p.ID.String()

		fmt.Printf("loading playground: %v\n", pIDStr)

		var err error
		for i, o := range p.Orders {
			orders[i], err = o.ToBacktesterOrder()
			if err != nil {
				return fmt.Errorf("loadPlaygrounds: failed to convert order: %w", err)
			}
		}

		var source *CreateAccountRequestSource
		var clockRequest CreateClockRequest
		if p.Env == "simulator" {
			if p.EndAt == nil {
				return fmt.Errorf("loadPlaygrounds: missing end date for simulator playground")
			}

			clockRequest = CreateClockRequest{
				StartDate: p.StartAt.Format(time.RFC3339),
				StopDate:  p.EndAt.Format(time.RFC3339),
			}

		} else if p.Env == "live" {
			if p.Broker == nil || p.AccountID == nil || p.LiveAccountType == nil {
				return fmt.Errorf("loadPlaygrounds: missing broker, account id, or api key for live playground")
			}

			liveAccountType := models.LiveAccountType(*p.LiveAccountType)
			if err := liveAccountType.Validate(); err != nil {
				return fmt.Errorf("loadPlaygrounds: invalid live account type: %w", err)
			}

			source = &CreateAccountRequestSource{
				Broker:      *p.Broker,
				AccountID:   *p.AccountID,
				AccountType: liveAccountType,
			}

			clockRequest = CreateClockRequest{
				StartDate: p.StartAt.Format(time.RFC3339),
			}

		} else {
			return fmt.Errorf("loadPlaygrounds: unknown environment: %v", p.Env)
		}

		var createRepoRequests []eventmodels.CreateRepositoryRequest
		for _, r := range p.Repositories {
			req, err := r.ToCreateRepositoryRequest()
			if err != nil {
				return fmt.Errorf("loadPlaygrounds: failed to convert repository: %w", err)
			}

			createRepoRequests = append(createRepoRequests, req)
		}

		var plot []*eventmodels.EquityPlot
		for _, r := range p.EquityPlotRecords {
			plot = append(plot, &eventmodels.EquityPlot{
				Timestamp: r.Timestamp,
				Value:     r.Equity,
			})
		}

		playground, err := CreatePlayground(&CreatePlaygroundRequest{
			ID:       &p.ID,
			ClientID: p.ClientID,
			Env:      p.Env,
			Account: CreateAccountRequest{
				Balance: p.Balance,
				Source:  source,
			},
			InitialBalance:    p.StartingBalance,
			Clock:             clockRequest,
			Repositories:      createRepoRequests,
			BackfillOrders:    orders,
			CreatedAt:         p.CreatedAt,
			EquityPlotRecords: plot,
			SaveToDB:          false,
		})

		if err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to create playground: %w", err)
		}

		playgrounds[playground.GetId()] = playground
	}

	return nil
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
		w.WriteHeader(404)
	} else if r.Method == "POST" {
		// handleCreatePlayground(w, r)
		w.WriteHeader(404)
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

type orderCache struct {
	container map[uint]models.OrderExecutionRequest
	mutex     *sync.Mutex
}

func (c *orderCache) Add(order *eventmodels.TradierOrder, entry models.OrderExecutionRequest) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.container[order.ID] = entry
}

func (c *orderCache) Get(order *eventmodels.TradierOrder) (models.OrderExecutionRequest, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry, ok := c.container[order.ID]
	return entry, ok
}

func (c *orderCache) GetMap() (container map[uint]models.OrderExecutionRequest, unlockFn func()) {
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

func handleLiveOrders(ctx context.Context, orderUpdateQueue *eventmodels.FIFOQueue[*eventmodels.TradierOrderUpdateEvent]) {
	cache := &orderCache{
		container: make(map[uint]models.OrderExecutionRequest),
		mutex:     &sync.Mutex{},
	}

	commitPendingOrders := func() {
		orderCache, unlockFn := cache.GetMap()

		defer unlockFn()

		for tradierOrder, orderFillEntry := range orderCache {
			playground, order, found := findOrder(tradierOrder)
			if !found {
				log.Errorf("handleLiveOrders: order not found: %v", tradierOrder)
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
			if err := saveOrderRecord(playground.GetId(), order, &balance, playground.GetLiveAccountType()); err != nil {
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
					continue
				}

				if event.CreateOrder != nil {
					if event.CreateOrder.Order.Status == string(models.BacktesterOrderStatusFilled) {
						cache.Add(event.CreateOrder.Order, models.OrderExecutionRequest{
							Time:     event.CreateOrder.Order.CreateDate,
							Price:    event.CreateOrder.Order.AvgFillPrice,
							Quantity: event.CreateOrder.Order.GetLastFillQuantity(),
						})

						log.Debugf("handleLiveOrders: order filled: %v", event.CreateOrder.Order)
					} else if event.CreateOrder.Order.Status == string(models.BacktesterOrderStatusPending) {
						log.Debugf("handleLiveOrders: order pending: %v", event.CreateOrder.Order)
					} else {
						log.Fatalf("handleLiveOrders: unknown order status: %v", event.CreateOrder.Order.Status)
					}

				} else if event.ModifyOrder != nil {
					if event.ModifyOrder.Field == "status" {
						playground, order, found := findOrder(event.ModifyOrder.OrderID)

						if found {
							reason, ok := event.ModifyOrder.New.(string)
							if !ok {
								log.Errorf("handleLiveOrders: failed to convert reason to string: %v", event.ModifyOrder.New)
								continue
							}

							if err := playground.RejectOrder(order, reason); err != nil {
								log.Errorf("handleLiveOrders: failed to reject order: %v", err)
							}

							if err := saveOrderRecord(playground.GetId(), order, nil, playground.GetLiveAccountType()); err != nil {
								log.Fatalf("handleLiveOrders: failed to save order record: %v", err)
							}
						} else {
							log.Warnf("handleLiveOrders: order not found: %v", event.CreateOrder.Order)
						}
					}
				} else if event.DeleteOrder != nil {
					// pass
				} else {
					log.Warnf("handleLiveOrders: unknown event type: %v", event)
				}

				time.Sleep(50 * time.Millisecond)
			}
		}
	}()
}

func SetupHandler(ctx context.Context, router *mux.Router, projectsDir string, apiKey string, ordersUpdateQueue *eventmodels.FIFOQueue[*eventmodels.TradierOrderUpdateEvent], database *gorm.DB) error {
	client = eventservices.NewPolygonTickDataMachine(apiKey)
	db = database
	projectsDirectory = projectsDir

	// needs to be async
	// how would i distribute the load?
	if err := loadPlaygrounds(); err != nil {
		return fmt.Errorf("SetupHandler: failed to load playgrounds: %w", err)
	}

	router.HandleFunc("", handlePlayground)
	router.HandleFunc("/{id}/account", handleAccount)
	router.HandleFunc("/{id}/order", handleOrder)
	router.HandleFunc("/{id}/tick", handleTick)
	router.HandleFunc("/{id}/candles", handleCandles)

	handleLiveOrders(ctx, ordersUpdateQueue)

	return nil
}
