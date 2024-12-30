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

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventpubsub"
	"github.com/jiaming2012/slack-trading/src/utils"
)

type TradierOrdersMonitoringWorker struct {
	wg                 *sync.WaitGroup
	orders             eventmodels.TradierOrderDataStore
	brokerURL          string
	timeSalesURL       string
	quotesBearerToken  string
	tradesBearerToken  string
	liveTradierCandles chan eventmodels.TradierMarketsTimeSalesDTO
	location           *time.Location
	currentBar         *eventmodels.TradierMarketsTimeSalesDTO
}

func (w *TradierOrdersMonitoringWorker) GetOrAddOrder(order *eventmodels.TradierOrder) (*eventmodels.TradierOrder, *eventmodels.TradierOrderCreateEvent) {
	if order, ok := w.orders[order.ID]; ok {
		return order, nil
	}

	w.orders.Add(order)

	return order, &eventmodels.TradierOrderCreateEvent{
		Order: order,
	}
}

func (w *TradierOrdersMonitoringWorker) FetchCandles(symbol eventmodels.Instrument, interval eventmodels.TradierInterval, start, end time.Time) ([]*eventmodels.TradierMarketsTimeSalesDTO, error) {
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
					Volume:    int(data["volume"].(float64)),
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
						Volume:    int(d["volume"].(float64)),
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

func (w *TradierOrdersMonitoringWorker) fetchOrders() ([]*eventmodels.TradierOrderDTO, error) {
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

func (w *TradierOrdersMonitoringWorker) CheckForDelete(ordersDTO []*eventmodels.TradierOrderDTO) []uint {
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

func (w *TradierOrdersMonitoringWorker) CheckForCreateOrUpdate(ordersDTO []*eventmodels.TradierOrderDTO) ([]*eventmodels.TradierOrderCreateEvent, []*eventmodels.TradierOrderUpdateEvent) {
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

func (w *TradierOrdersMonitoringWorker) OrdersMonitoringWorking(ctx context.Context) {
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

func (w *TradierOrdersMonitoringWorker) FetchLiveTradierCandles() {
	start := time.Now().Add(-1 * time.Hour).In(w.location)
	end := start.Add(24 * time.Hour)

	bars, err := w.FetchCandles(eventmodels.StockSymbol("COIN"), eventmodels.TradierInterval1Min, start, end)
	if err != nil {
		log.Fatalf("failed to fetch candles: %v", err)
		return
	}

	if len(bars) > 1 {
		newBar := bars[len(bars)-2]

		if w.currentBar == nil {
			w.currentBar = newBar
			w.liveTradierCandles <- *newBar
		} else {
			if w.currentBar.Timestamp != (newBar.Timestamp) {
				w.currentBar = newBar
				w.liveTradierCandles <- *newBar
			}
		}
	}
}

func (w *TradierOrdersMonitoringWorker) Start(ctx context.Context) {
	w.wg.Add(1)

	timer := time.NewTicker(5 * time.Second)

	go func() {
		defer w.wg.Done()
		for {
			select {
			case <-ctx.Done():
				log.Info("stopping TradierOrdersMonitoringWorker consumer")
				return
			case <-timer.C:
				// w.OrdersMonitoringWorking(ctx)
				if w.liveTradierCandles != nil {
					w.FetchLiveTradierCandles()
				}
			}
		}
	}()
}

func NewTradierOrdersMonitoringWorker(wg *sync.WaitGroup, liveTradierCandles chan eventmodels.TradierMarketsTimeSalesDTO, brokerURL, timeSalesURL, quotesBearerToken, tradesBearerToken string) *TradierOrdersMonitoringWorker {
	worker := &TradierOrdersMonitoringWorker{
		wg:                 wg,
		orders:             make(map[uint]*eventmodels.TradierOrder),
		brokerURL:          brokerURL,
		timeSalesURL:       timeSalesURL,
		quotesBearerToken:  quotesBearerToken,
		tradesBearerToken:  tradesBearerToken,
		liveTradierCandles: liveTradierCandles,
	}

	var err error

	worker.location, err = time.LoadLocation("America/New_York")

	if err != nil {
		log.Fatalf("failed to load location America/New_York: %v", err)
	}

	return worker
}
