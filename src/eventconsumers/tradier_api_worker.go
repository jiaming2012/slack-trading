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
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/backtester-api/services"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

type TradierApiWorker struct {
	wg                *sync.WaitGroup
	db                *gorm.DB
	orders            eventmodels.TradierOrderDataStore
	brokerURL         string
	timeSalesURL      string
	quotesBearerToken string
	tradesBearerToken string
	location          *time.Location
	polygonClient     *eventservices.PolygonTickDataMachine
	tradesUpdateQueue *eventmodels.FIFOQueue[*eventmodels.TradierOrderUpdateEvent]
}

func (w *TradierApiWorker) getOrAddOrder(order *eventmodels.TradierOrder) (*eventmodels.TradierOrder, *eventmodels.TradierOrderCreateEvent) {
	if order, ok := w.orders[order.ID]; ok {
		return order, nil
	}

	w.orders.Add(order)

	return order, &eventmodels.TradierOrderCreateEvent{
		Order: order,
	}
}

func (w *TradierApiWorker) fetchTradierCandles(symbol eventmodels.Instrument, interval eventmodels.TradierInterval, start, end time.Time) ([]*eventmodels.TradierMarketsTimeSalesDTO, error) {
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
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:FetchCandles(): failed to fetch option prices: %s", res.Status)
	}

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:FetchCandles(): failed to read response body: %w", err)
	}

	var respMap map[string]interface{}

	if json.Unmarshal(bytes, &respMap) != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:FetchCandles(): failed to parse response body: %w", err)
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
				return nil, fmt.Errorf("TradierOrdersMonitoringWorker:FetchCandles(): expected data to be a map or a list of maps, got %T", series)
			}

		} else {
			return nil, fmt.Errorf("TradierOrdersMonitoringWorker:FetchCandles(): expected series to be a map, got %T", series)
		}
	}

	return results, nil
}

func (w *TradierApiWorker) fetchOrder(orderID uint) (*eventmodels.TradierOrderDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	queryParams := url.Values{}
	queryParams.Add("includeTags", "true")

	url := fmt.Sprintf("%s/%d?%s", w.brokerURL, orderID, queryParams.Encode())

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrder(): failed to create request: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", w.tradesBearerToken))

	log.Debugf("fetching order from %s", req.URL.String())

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrder(): failed to fetch option prices: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrder(): failed to fetch option prices: %s", res.Status)
	}

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrder(): failed to read response body: %w", err)
	}

	var resp eventmodels.TradierFetchOrderResponse

	if err := json.Unmarshal(bytes, &resp); err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrder(): failed to parse response body: %w", err)
	}

	return resp.Order, nil
}

func (w *TradierApiWorker) fetchOrders() ([]*eventmodels.TradierOrderDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	queryParams := url.Values{}
	queryParams.Add("includeTags", "true")

	url := fmt.Sprintf("%s?%s", w.brokerURL, queryParams.Encode())

	req, err := http.NewRequest(http.MethodGet, url, nil)
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

func (w *TradierApiWorker) checkForDelete(ordersDTO []*eventmodels.TradierOrderDTO) []uint {
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

func (w *TradierApiWorker) checkForCreateOrUpdate(ordersDTO []*eventmodels.TradierOrderDTO) ([]*eventmodels.TradierOrderCreateEvent, []*eventmodels.TradierOrderModifyEvent) {
	var createOrderEvents []*eventmodels.TradierOrderCreateEvent
	var updateOrderEvents []*eventmodels.TradierOrderModifyEvent

	for _, orderDTO := range ordersDTO {
		newOrder, err := orderDTO.ToTradierOrder()
		if err != nil {
			log.Errorf("TradierOrdersMonitoringWorker.CheckForCreateOrUpdate: failed to convert order DTO to order: %v", err)
			continue
		}

		_, createOrderEvent := w.getOrAddOrder(newOrder)
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

func (w *TradierApiWorker) fetchPendingOrdersfromDB() ([]*models.OrderRecord, error) {
	var orders []*models.OrderRecord

	if err := w.db.Where("status = ?", "pending").Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch pending orders: %w", err)
	}

	return orders, nil
}

func (w *TradierApiWorker) executeOrdersQueueUpdate(ctx context.Context) {
	pendingOrders, err := w.fetchPendingOrdersfromDB()
	if err != nil {
		log.Errorf("TradierOrdersMonitoringWorker.Start: failed to fetch orders: %v", err)
		return
	}

	// log.Debugf("TradierApiWorker.executeOrdersQueueUpdate: fetched %d pending orders", len(pendingOrders))

	for _, order := range pendingOrders {
		orderDTO, err := w.fetchOrder(order.ExternalOrderID)
		if err != nil {
			log.Errorf("TradierOrdersMonitoringWorker.Start: failed to fetch order: %v", err)
			continue
		}

		log.Debugf("orderDTO: %v", orderDTO)

		if orderDTO.Status == string(models.BacktesterOrderStatusFilled) {
			o, err := orderDTO.ToTradierOrder()
			if err != nil {
				log.Errorf("TradierApiWorker.executeOrdersQueueUpdate: failed to convert order to backtester order: %v", err)
			}

			w.tradesUpdateQueue.Enqueue(&eventmodels.TradierOrderUpdateEvent{
				CreateOrder: &eventmodels.TradierOrderCreateEvent{
					Order: o,
				},
			})

			log.Debugf("TradierApiWorker.executeOrdersQueueUpdate: order %d is filled", order.ExternalOrderID)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func (w *TradierApiWorker) getStartEndDates(lastTimestamp, now time.Time, period time.Duration) (time.Time, time.Time) {
	startAfter := lastTimestamp.In(w.location)

	start := startAfter.Truncate(period)

	endAfter := now.In(w.location)

	end := endAfter.Truncate(period)

	return start, end
}

func (w *TradierApiWorker) updateLiveRepos(repo *models.CandleRepository) {
	now := time.Now()
	period := repo.GetPeriod()
	periodStr := period.String()

	log.Debugf("updating live repo %s with period %s", repo.GetSymbol(), periodStr)

	lastCandleInRepo := repo.GetLastCandle()

	start, end := w.getStartEndDates(lastCandleInRepo.Timestamp, now, period)

	var candles []eventmodels.ICandle

	if period <= 15*time.Minute {
		tradierCandles, err := w.fetchTradierCandles(repo.GetSymbol(), repo.GetFetchInterval(), start, end)
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

	cutoffTimestamp := now.Truncate(period)

	startAt := len(candles)

	if lastCandleInRepo != nil {
		for i := len(candles) - 1; i >= 0; i-- {
			tstamp := candles[i].GetTimestamp()
			if !tstamp.After(lastCandleInRepo.Timestamp) {
				break
			}

			startAt = i
		}
	}
	var newCandles []eventmodels.ICandle
	for i := startAt; i < len(candles); i++ {
		if !candles[i].GetTimestamp().Before(cutoffTimestamp) {
			break
		}

		timestamp := candles[i].GetTimestamp()
		totalMinutes := timestamp.Unix() / 60
		periodInMinutes := int64(period.Minutes())
		if totalMinutes%periodInMinutes == 0 {
			newCandles = append(newCandles, candles[i])
		}
	}

	_ = skipCandles

	repo.AppendBars(newCandles)
}

func (w *TradierApiWorker) ExecuteLiveReposUpdate() {
	repos, unlockFn, err := services.FetchAllLiveRepositories()
	defer unlockFn()

	if err != nil {
		log.Errorf("failed to fetch all live repositories: %v", err)
		return
	}

	for _, repo := range repos {
		r := repo
		log.Debugf("fetching candles for %s", r.GetSymbol())
		go w.updateLiveRepos(r)
	}
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
				w.executeOrdersQueueUpdate(ctx)
				w.ExecuteLiveReposUpdate()
			}
		}
	}()
}

func NewTradierApiWorker(wg *sync.WaitGroup, brokerURL, timeSalesURL, quotesBearerToken, tradesBearerToken string, polygonClient *eventservices.PolygonTickDataMachine, tradesUpdateQueue *eventmodels.FIFOQueue[*eventmodels.TradierOrderUpdateEvent], db *gorm.DB) *TradierApiWorker {
	worker := &TradierApiWorker{
		wg:                wg,
		db:                db,
		orders:            make(map[uint]*eventmodels.TradierOrder),
		brokerURL:         brokerURL,
		timeSalesURL:      timeSalesURL,
		quotesBearerToken: quotesBearerToken,
		tradesBearerToken: tradesBearerToken,
		polygonClient:     polygonClient,
		tradesUpdateQueue: tradesUpdateQueue,
	}

	var err error

	worker.location, err = time.LoadLocation("America/New_York")

	if err != nil {
		log.Fatalf("failed to load location America/New_York: %v", err)
	}

	return worker
}
