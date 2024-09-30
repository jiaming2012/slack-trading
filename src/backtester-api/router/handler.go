package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
)

// todo: move state to database
var (
	client      = new(eventservices.PolygonTickDataMachine)
	playgrounds = map[uuid.UUID]*models.Playground{}
	repos       = map[uuid.UUID]*models.BacktesterCandleRepository{}
	clocks      = map[uuid.UUID]*models.Clock{}
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

func setResponse(response map[string]interface{}, w http.ResponseWriter) error {
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

type CreateRepositoryRequest struct {
	Symbol   string                 `json:"symbol"`
	Timespan PolygonTimespanRequest `json:"timespan"`
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

	repository, err := createRepository(eventmodels.StockSymbol(req.Repository.Symbol), timespan, from, to)
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
		"playground": playground.ID,
	}

	if err := setResponse(response, w); err != nil {
		setErrorResponse("createPlayground: failed to set response", 500, err, w)
		return
	}
}

func createClock(start, stop *eventmodels.PolygonDate) (*models.Clock, error) {
	startTime, err := time.Parse("2006-01-02", start.ToString())
	if err != nil {
		return nil, fmt.Errorf("createClock: failed to parse start date: %w", err)
	}

	endTime, err := time.Parse("2006-01-02", stop.ToString())
	if err != nil {
		return nil, fmt.Errorf("createClock: failed to parse end date: %w", err)
	}

	return models.NewClock(startTime, endTime), nil
}

func createRepository(symbol eventmodels.StockSymbol, timespan eventmodels.PolygonTimespan, from, to *eventmodels.PolygonDate) (*models.BacktesterCandleRepository, error) {
	bars, err := client.FetchAggregateBars(symbol, timespan, from, to)
	if err != nil {
		return nil, fmt.Errorf("createRepository: failed to fetch aggregate bars: %w", err)
	}

	return models.NewBacktesterCandleRepository(symbol, bars), nil
}

func handleAccount(w http.ResponseWriter, r *http.Request) {
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

	response := map[string]interface{}{
		"balance": playground.GetBalance(),
		"orders":  playground.GetOrders(),
		"positions": playground.GetPositions(),
	}

	if err := setResponse(response, w); err != nil {
		setErrorResponse("handleAccount: failed to set response", 500, err, w)
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

func SetupHandler(router *mux.Router, apiKey string) {
	client = eventservices.NewPolygonTickDataMachine(apiKey)

	router.HandleFunc("", handlePlayground)
	router.HandleFunc("/{id}/account", handleAccount)
}
