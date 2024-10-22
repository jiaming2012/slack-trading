package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/backtester-api/services"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

// todo: move state to database
var (
	client            = new(eventservices.PolygonTickDataMachine)
	playgrounds       = map[uuid.UUID]*models.Playground{}
	projectsDirectory string
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

type CreatePlaygroundRequest struct {
	Balance    float64                 `json:"balance"`
	Clock      CreateClockRequest      `json:"clock"`
	Repository CreateRepositoryRequest `json:"repository"`
}

type CreateClockRequest struct {
	StartDate string `json:"start"`
	StopDate  string `json:"stop"`
}

type PolygonTimespanRequest struct {
	Multiplier int    `json:"multiplier"`
	Unit       string `json:"unit"`
}

type RepositorySourceType string

const (
	RepositorySourcePolygon RepositorySourceType = "polygon"
	RepositorySourceCSV     RepositorySourceType = "csv"
)

type RepositorySource struct {
	Type        RepositorySourceType `json:"type"`
	CSVFilename *string              `json:"filename"`
}

type CreateRepositoryRequest struct {
	Symbol   string                 `json:"symbol"`
	Timespan PolygonTimespanRequest `json:"timespan"`
	Source   RepositorySource       `json:"source"`
}

type CreateOrderRequest struct {
	Symbol    string                         `json:"symbol"`
	Class     models.BacktesterOrderClass    `json:"class"`
	Quantity  float64                        `json:"quantity"`
	Side      models.BacktesterOrderSide     `json:"side"`
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

func placeOrder(playground *models.Playground, req *CreateOrderRequest, createdOn time.Time) (*models.BacktesterOrder, error) {
	order := models.NewBacktesterOrder(
		playground.NextOrderID(),
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

	if err := playground.PlaceOrder(order); err != nil {
		return nil, fmt.Errorf("placeOrder: failed to place order: %w", err)
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

	playground, ok := playgrounds[id]
	if !ok {
		setErrorResponse("handleAccount: playground not found", 404, fmt.Errorf("playground not found"), w)
		return
	}

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		setErrorResponse("createOrder: failed to decode request", 400, err, w)
		return
	}

	if err := req.Validate(); err != nil {
		setErrorResponse("createOrder: invalid request", 400, err, w)
		return
	}

	createdOn := playground.GetCurrentTime()

	order, err := placeOrder(playground, &req, createdOn)
	if err != nil {
		setErrorResponse("createOrder: failed to place order", 500, err, w)
		return
	}

	if err := setResponse(order, w); err != nil {
		setErrorResponse("createOrder: failed to set response", 500, err, w)
		return
	}
}

func createPlayground(w http.ResponseWriter, r *http.Request) {
	var req CreatePlaygroundRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		setErrorResponse("createClock: failed to decode request", 400, err, w)
		return
	}

	// create clock
	from, err := eventmodels.NewPolygonDate(req.Clock.StartDate)
	if err != nil {
		setErrorResponse("createPlayground: failed to parse clock.startDate", 400, err, w)
		return
	}

	to, err := eventmodels.NewPolygonDate(req.Clock.StopDate)
	if err != nil {
		setErrorResponse("createPlayground: failed to parse to clock.endDate", 400, err, w)
		return
	}

	clock, err := createClock(from, to)
	if err != nil {
		setErrorResponse("createPlayground: failed to create clock", 500, err, w)
		return
	}

	// create repository
	timespan := eventmodels.PolygonTimespan{
		Multiplier: req.Repository.Timespan.Multiplier,
		Unit:       eventmodels.PolygonTimespanUnit(req.Repository.Timespan.Unit),
	}

	var bars []*eventmodels.PolygonAggregateBarV2
	if req.Repository.Source.Type == RepositorySourcePolygon {
		bars, err = client.FetchAggregateBars(eventmodels.StockSymbol(req.Repository.Symbol), timespan, from, to)
		if err != nil {
			setErrorResponse("createPlayground: failed to fetch aggregate bars", 500, err, w)
			return
		}
	} else if req.Repository.Source.Type == RepositorySourceCSV {
		if req.Repository.Source.CSVFilename == nil {
			setErrorResponse("createPlayground: missing CSV filename", 400, fmt.Errorf("missing CSV filename"), w)
			return
		}

		sourceDir := path.Join(projectsDirectory, "slack-trading", "src", "backtester-api", "data", *req.Repository.Source.CSVFilename)

		bars, err = utils.ImportCandlesFromCsv(sourceDir)
		if err != nil {
			setErrorResponse("createPlayground: failed to import candles from CSV", 500, err, w)
			return
		}
	} else {
		setErrorResponse("createPlayground: invalid repository source", 400, fmt.Errorf("invalid repository source"), w)
		return
	}

	repository, err := createRepository(eventmodels.StockSymbol(req.Repository.Symbol), timespan, bars)
	if err != nil {
		setErrorResponse("createPlayground: failed to create repository", 500, err, w)
		return
	}

	// create playground
	playground, err := models.NewPlayground(req.Balance, clock, repository)
	if err != nil {
		setErrorResponse("createPlayground: failed to create playground", 500, err, w)
		return
	}

	playgrounds[playground.ID] = playground

	response := map[string]interface{}{
		"playground_id": playground.ID,
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

func createRepository(symbol eventmodels.StockSymbol, timespan eventmodels.PolygonTimespan, bars []*eventmodels.PolygonAggregateBarV2) (*models.BacktesterCandleRepository, error) {
	return models.NewBacktesterCandleRepository(symbol, bars), nil
}

func convertPositionsToMap(positions map[eventmodels.Instrument]*models.Position) map[string]interface{} {
	response := map[string]interface{}{}

	for k, v := range positions {
		response[k.GetTicker()] = v
	}

	return response
}

func getAccountInfo(playground *models.Playground, fetchOrders bool) map[string]interface{} {
	positions := playground.GetPositions()
	out := map[string]interface{}{
		"balance":     playground.GetBalance(),
		"equity":      playground.GetEquity(positions),
		"free_margin": playground.GetFreeMarginFromPositionMap(positions),
		"positions":   convertPositionsToMap(positions),
	}

	if fetchOrders {
		out["orders"] = playground.GetOrders()
	}

	return out
}

func handleAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(404)
		return
	}

	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		setErrorResponse("handleAccount: failed to playground id", 400, err, w)
		return
	}

	playground, ok := playgrounds[id]
	if !ok {
		setErrorResponse("handleAccount: playground not found", 404, fmt.Errorf("playground not found"), w)
		return
	}

	fetchOrders := true
	if r.URL.Query().Get("orders") == "false" {
		fetchOrders = false
	}

	accountInfo := getAccountInfo(playground, fetchOrders)

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

	playground, ok := playgrounds[id]
	if !ok {
		setErrorResponse("handleCandles: playground not found", 404, fmt.Errorf("playground not found"), w)
		return
	}

	// fetch from query parameters
	if err := r.ParseForm(); err != nil {
		setErrorResponse("handleCandles: failed to parse form", 400, err, w)
		return
	}

	symbolStr := r.Form.Get("symbol")
	if symbolStr == "" {
		setErrorResponse("handleCandles: missing symbol", 400, fmt.Errorf("missing symbol"), w)
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

	candles, err := playground.FetchCandles(eventmodels.StockSymbol(symbolStr), from, to)
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
		createPlayground(w, r)
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

	seconds, err := time.ParseDuration(fmt.Sprintf("%ss", secondsStr))
	if err != nil {
		setErrorResponse("handleTick: failed to parse seconds", 500, err, w)
		return
	}

	// get playground id
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		setErrorResponse("handleTick: failed to playground id", 400, err, w)
		return
	}

	playground, ok := playgrounds[id]
	if !ok {
		setErrorResponse("handleTick: playground not found", 404, fmt.Errorf("playground not found"), w)
		return
	}

	// tick
	stateChange, err := playground.Tick(seconds, preview)
	if err != nil {
		setErrorResponse("handleTick: failed to tick", 500, err, w)
		return
	}

	if err := setResponse(stateChange, w); err != nil {
		setErrorResponse("handleTick: failed to set response", 500, err, w)
		return
	}
}

func SetupHandler(router *mux.Router, projectsDir string, apiKey string) {
	client = eventservices.NewPolygonTickDataMachine(apiKey)
	projectsDirectory = projectsDir

	router.HandleFunc("", handlePlayground)
	router.HandleFunc("/{id}/account", handleAccount)
	router.HandleFunc("/{id}/order", handleOrder)
	router.HandleFunc("/{id}/tick", handleTick)
	router.HandleFunc("/{id}/candles", handleCandles)
}
