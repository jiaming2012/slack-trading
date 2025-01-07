package eventconsumers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/backtester-api/services"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventpubsub"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

type TradierApiWorker struct {
	wg                *sync.WaitGroup
	orders            eventmodels.TradierOrderDataStore
	brokerURL         string
	timeSalesURL      string
	quotesBearerToken string
	tradesBearerToken string
	location          *time.Location
	polygonClient     *eventservices.PolygonTickDataMachine
}

func (w *TradierApiWorker) GetOrAddOrder(order *eventmodels.TradierOrder) (*eventmodels.TradierOrder, *eventmodels.TradierOrderCreateEvent) {
	if order, ok := w.orders[order.ID]; ok {
		return order, nil
	}

	w.orders.Add(order)

	return order, &eventmodels.TradierOrderCreateEvent{
		Order: order,
	}
}

func (w *TradierApiWorker) FetchTradierCandles(symbol eventmodels.Instrument, interval eventmodels.TradierInterval, start, end time.Time) ([]*eventmodels.TradierMarketsTimeSalesDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	// Construct the query parameters
	params := url.Values{}
	params.Add("symbol", symbol.GetTicker())
	params.Add("interval", string(interval))
	params.Add("start", start.Format("2006-01-02 15:04"))
	params.Add("end", end.Format("2006-01-02 15:04"))

	url := fmt.Sprintf("%s?%s", w.timeSalesURL, params.Encode())

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:FetchCandles(): failed to create request: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", w.quotesBearerToken))
	log.Debugf("fetching candles from %s", req.URL.String())

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:FetchCandles(): failed to fetch option prices: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to fetch option prices: %s", res.Status)
	}

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to read response body: %w", err)
	}

	var respMap map[string]interface{}

	if json.Unmarshal(bytes, &respMap) != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to parse response body: %w", err)
	}

	var results []*eventmodels.TradierMarketsTimeSalesDTO
	if series, ok := respMap["series"]; ok {
		if series == nil {
			return results, nil
		}

		if seriesSingleton, ok := series.(map[string]interface{}); ok {
			if data, isSingleton := seriesSingleton["data"].(map[string]interface{}); isSingleton {
				results = append(results, &eventmodels.TradierMarketsTimeSalesDTO{
					Time:      data["time"].(string),
					Timestamp: int(data["timestamp"].(float64)),
					Price:     data["price"].(float64),
					Open:      data["open"].(float64),
					High:      data["high"].(float64),
					Low:       data["low"].(float64),
					Close:     data["close"].(float64),
					Volume:    data["volume"].(float64),
					Vwap:      data["vwap"].(float64),
				})
			} else if data, isList := seriesSingleton["data"].([]interface{}); isList {
				for _, obj := range data {
					d := obj.(map[string]interface{})
					results = append(results, &eventmodels.TradierMarketsTimeSalesDTO{
						Time:      d["time"].(string),
						Timestamp: int(d["timestamp"].(float64)),
						Price:     d["price"].(float64),
						Open:      d["open"].(float64),
						High:      d["high"].(float64),
						Low:       d["low"].(float64),
						Close:     d["close"].(float64),
						Volume:    d["volume"].(float64),
						Vwap:      d["vwap"].(float64),
					})
				}
			} else {
				return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): expected data to be a map or a list of maps, got %T", series)
			}

		} else {
			return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): expected series to be a map, got %T", series)
		}
	}

	return results, nil
}

func (w *TradierApiWorker) fetchOrders() ([]*eventmodels.TradierOrderDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, w.brokerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to create request: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", w.tradesBearerToken))

	log.Debugf("fetching orders from %s", req.URL.String())

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to fetch option prices: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to fetch option prices: %s", res.Status)
	}

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to read response body: %w", err)
	}

	orders, err := utils.ParseTradierResponse[*eventmodels.TradierOrderDTO](bytes)
	if err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to parse response body: %w", err)
	}

	return orders, nil
}

func (w *TradierApiWorker) CheckForDelete(ordersDTO []*eventmodels.TradierOrderDTO) []uint {
	result := []uint{}

	for orderID := range w.orders {
		found := false
		for _, orderDTO := range ordersDTO {
			if orderDTO.ID == orderID {
				found = true
				break
			}
		}

		if !found {
			result = append(result, orderID)
		}
	}

	return result
}

func (w *TradierApiWorker) CheckForCreateOrUpdate(ordersDTO []*eventmodels.TradierOrderDTO) ([]*eventmodels.TradierOrderCreateEvent, []*eventmodels.TradierOrderUpdateEvent) {
	var createOrderEvents []*eventmodels.TradierOrderCreateEvent
	var updateOrderEvents []*eventmodels.TradierOrderUpdateEvent

	for _, orderDTO := range ordersDTO {
		newOrder, err := orderDTO.ToTradierOrder()
		if err != nil {
			log.Errorf("TradierOrdersMonitoringWorker.CheckForCreateOrUpdate: failed to convert order DTO to order: %v", err)
			continue
		}

		_, createOrderEvent := w.GetOrAddOrder(newOrder)
		if createOrderEvent != nil {
			createOrderEvents = append(createOrderEvents, createOrderEvent)
		} else {
			updates := w.orders.Update(newOrder)
			if len(updates) > 0 {
				updateOrderEvents = append(updateOrderEvents, updates...)
			}
		}
	}

	return createOrderEvents, updateOrderEvents
}

func (w *TradierApiWorker) OrdersMonitoringWorking(ctx context.Context) {
	ordersDTO, err := w.fetchOrders()
	if err != nil {
		log.Errorf("TradierOrdersMonitoringWorker.Start: failed to fetch orders: %v", err)
		return
	}

	// check for delete
	orderIDs := w.CheckForDelete(ordersDTO)
	for _, orderID := range orderIDs {
		w.orders.Delete(orderID)
		eventpubsub.PublishEvent("TradierOrdersMonitoringWorker", eventmodels.TradierOrderDeleteEventName, &eventmodels.TradierOrderDeleteEvent{
			OrderID: orderID,
		})
	}

	// check for add or update
	createOrderEvents, updateEvents := w.CheckForCreateOrUpdate(ordersDTO)
	for _, orderEvent := range createOrderEvents {
		eventpubsub.PublishEvent("TradierOrdersMonitoringWorker", eventmodels.TradierOrderCreateEventName, orderEvent)
	}

	for _, updateEvent := range updateEvents {
		eventpubsub.PublishEvent("TradierOrdersMonitoringWorker", eventmodels.TradierOrderUpdateEventName, updateEvent)
	}
}

func (w *TradierApiWorker) UpdateLiveRepos(repo *models.CandleRepository) {
	now := time.Now()
	start := now.Add(-24 * time.Hour).In(w.location)
	end := now.Add(24 * time.Hour)

	var candles []eventmodels.ICandle

	if repo.GetPeriod() <= 15*time.Minute {
		tradierCandles, err := w.FetchTradierCandles(repo.GetSymbol(), repo.GetFetchInterval(), start, end)
		if err != nil {
			log.Errorf("failed to fetch candles: %v", err)
			return
		}

		for _, candle := range tradierCandles {
			candles = append(candles, candle)
		}
	} else {
		polygonCandles, err := w.polygonClient.FetchAggregateBarsWithDates(repo.GetSymbol(), repo.GetPolygonTimespan(), start, end, w.location)
		if err != nil {
			log.Errorf("failed to fetch candles: %v", err)
			return
		}

		for _, candle := range polygonCandles {
			candles = append(candles, candle)
		}
	}

	startAt := len(candles)
	var newCandles []eventmodels.ICandle
	lastCandleInRepo := repo.GetLastCandle()
	skipCandles := 1
	if lastCandleInRepo != nil {
		for i := len(candles) - skipCandles - 1; i >= 0; i-- {
			tstamp := candles[i].GetTimestamp()
			if !tstamp.After(lastCandleInRepo.Timestamp) {
				break
			}

			startAt = i
		}
	}

	for i := startAt; i < len(candles)-skipCandles; i++ {
		timestamp := candles[i].GetTimestamp()
		totalMinutes := timestamp.Unix() / 60
		period := int64(repo.GetPeriod().Minutes())
		if totalMinutes%period == 0 {
			newCandles = append(newCandles, candles[i])
		}
	}

	repo.AppendBars(newCandles)
}

func (w *TradierApiWorker) Start(ctx context.Context) {
	w.wg.Add(1)

	timer := time.NewTicker(10 * time.Second)

	go func() {
		defer w.wg.Done()
		for {
			select {
			case <-ctx.Done():
				log.Info("stopping TradierOrdersMonitoringWorker consumer")
				return
			case <-timer.C:
				// w.OrdersMonitoringWorking(ctx)
				repos, unlockFn, err := services.FetchAllLiveRepositories()
				if err != nil {
					log.Errorf("failed to fetch all live repositories: %v", err)
					continue
				}

				for _, repo := range repos {
					log.Debugf("fetching candles for %s", repo.GetSymbol())
					w.UpdateLiveRepos(repo)
				}

				unlockFn()
			}
		}
	}()
}

func NewTradierApiWorker(wg *sync.WaitGroup, candlesQueue *eventmodels.FIFOQueue[*eventmodels.TradierCandleUpdate], brokerURL, timeSalesURL, quotesBearerToken, tradesBearerToken string, polygonClient *eventservices.PolygonTickDataMachine) *TradierApiWorker {
	worker := &TradierApiWorker{
		wg:                wg,
		orders:            make(map[uint]*eventmodels.TradierOrder),
		brokerURL:         brokerURL,
		timeSalesURL:      timeSalesURL,
		quotesBearerToken: quotesBearerToken,
		tradesBearerToken: tradesBearerToken,
		polygonClient:     polygonClient,
	}

	var err error

	worker.location, err = time.LoadLocation("America/New_York")

	if err != nil {
		log.Fatalf("failed to load location America/New_York: %v", err)
	}

	return worker
}
