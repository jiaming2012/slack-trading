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

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/backtester-api/services"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
)

type TradierApiWorker struct {
	wg                *sync.WaitGroup
	db                *gorm.DB
	dbService         models.IDatabaseService
	orders            models.TradierOrderDataStore
	timeSalesURL      string
	quotesBearerToken string
	location          *time.Location
	polygonClient     *eventservices.PolygonTickDataMachine
	tradesUpdateQueue *eventmodels.FIFOQueue[*models.TradierOrderUpdateEvent]
	calendarURL       string
}

func (w *TradierApiWorker) getOrAddOrder(order *eventmodels.TradierOrder) (*eventmodels.TradierOrder, *models.TradierOrderCreateEvent) {
	if order, ok := w.orders[order.ID]; ok {
		return order, nil
	}

	w.orders.Add(order)

	return order, &models.TradierOrderCreateEvent{
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
	log.Tracef("fetching tradier candles from %s", req.URL.String())

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:FetchCandles(): failed to fetch candles: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:FetchCandles(): failed to fetch candles: %s", res.Status)
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

// func (w *TradierApiWorker) fetchOrders() ([]*eventmodels.TradierOrderDTO, error) {
// 	client := http.Client{
// 		Timeout: 10 * time.Second,
// 	}

// 	queryParams := url.Values{}
// 	queryParams.Add("includeTags", "true")

// 	url := fmt.Sprintf("%s?%s", w.paperBrokerURL, queryParams.Encode())

// 	req, err := http.NewRequest(http.MethodGet, url, nil)
// 	if err != nil {
// 		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to create request: %w", err)
// 	}

// 	req.Header.Add("Accept", "application/json")
// 	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", w.paperTradesBearerToken))

// 	log.Debugf("fetching orders from %s", req.URL.String())

// 	res, err := client.Do(req)
// 	if err != nil {
// 		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to fetch orders: %w", err)
// 	}

// 	defer res.Body.Close()

// 	if res.StatusCode != http.StatusOK {
// 		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to fetch orders: %s", res.Status)
// 	}

// 	bytes, err := io.ReadAll(res.Body)
// 	if err != nil {
// 		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to read response body: %w", err)
// 	}

// 	orders, err := utils.ParseTradierResponse[*eventmodels.TradierOrderDTO](bytes)
// 	if err != nil {
// 		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to parse response body: %w", err)
// 	}

// 	return orders, nil
// }

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

func (w *TradierApiWorker) checkForCreateOrUpdate(ordersDTO []*eventmodels.TradierOrderDTO) ([]*models.TradierOrderCreateEvent, []*models.TradierOrderModifyEvent) {
	var createOrderEvents []*models.TradierOrderCreateEvent
	var updateOrderEvents []*models.TradierOrderModifyEvent

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

func (w *TradierApiWorker) updateTradierOrderQueue(ctx context.Context) {
	log.Trace("TradierApiWorker.executeOrdersQueueUpdate: begin ...")
	defer log.Trace("TradierApiWorker.executeOrdersQueueUpdate: end")

	sleepDuration := 10 * time.Millisecond
	if err := services.UpdateTradierOrderQueue(w.tradesUpdateQueue, w.dbService, sleepDuration); err != nil {
		log.Errorf("failed to update tradier order queue: %v", err)
	}

	if err := services.UpdatePendingMarginOrders(w.dbService); err != nil {
		log.Errorf("failed to update pending margin orders: %v", err)
	}
}

func (w *TradierApiWorker) getStartEndDates(lastTimestamp, now time.Time, period time.Duration) (time.Time, time.Time) {
	startAfter := lastTimestamp.In(w.location)

	start := startAfter.Truncate(period)

	endAfter := now.In(w.location)

	end := endAfter.Truncate(period)

	return start, end
}

func (w *TradierApiWorker) updateLiveRepos(playgroundId uuid.UUID, repo *models.CandleRepository) {
	now := time.Now()
	period := repo.GetPeriod()
	periodStr := period.String()

	symbol := repo.GetSymbol().GetTicker()

	log.Tracef("Playground id %s live repo update: fetching %s - %s candles", playgroundId, symbol, periodStr)

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

	maxTimestamp, err := repo.AppendBars(newCandles)
	if err != nil {
		log.Errorf("failed to append bars: %v", err)
		return
	}

	if len(newCandles) > 0 {
		log.Infof("Playground id %s: %s - %s: updated %d candles", playgroundId, repo.GetSymbol().GetTicker(), repo.GetPeriodStr(), len(newCandles))
	} else {
		log.Tracef("Playground id %s: %s - %s: no new candles", playgroundId, repo.GetSymbol().GetTicker(), repo.GetPeriodStr())
	}

	if !maxTimestamp.IsZero() {
		nextUpdateAt := repo.SetNextUpdateAt(maxTimestamp)
		log.Infof("Playground id %s: %s - %s: nextupdate at %s", playgroundId, symbol, periodStr, nextUpdateAt)
	}
}

func (w *TradierApiWorker) ExecuteLiveReposUpdate() {
	log.Trace("TradierApiWorker.ExecuteLiveReposUpdate: begin ...")
	defer log.Trace("TradierApiWorker.ExecuteLiveReposUpdate: end")

	now := time.Now()
	playgrounds := w.dbService.GetPlaygrounds()

	count := 0
	for _, playground := range playgrounds {
		if err := playground.GetLiveAccountType().Validate(); err == nil {
			repos := playground.GetRepositories()
			for _, repo := range repos {
				r := repo

				nextUpdateAt := r.GetNextUpdateAt()
				if nextUpdateAt == nil || now.After(*nextUpdateAt) {
					count += 1
					go w.updateLiveRepos(playground.GetId(), r)
				}
			}
		}
	}

	log.Debugf("TradierApiWorker.ExecuteLiveReposUpdate: updated %d repos", count)
}

func (w *TradierApiWorker) IsMarketOpen() bool {
	now := time.Now()
	nowEST := now.In(w.location)
	nowUTC := now.UTC()

	calendar, err := eventservices.FetchMarketCalendar(w.calendarURL, w.quotesBearerToken, nowUTC)
	if err != nil {
		log.Errorf("Failed to fetch market calendar: %v", err)
		return false
	}

	open, err := eventservices.IsMarketOpen(calendar, nowEST)
	if err != nil {
		log.Errorf("Failed to check if market is open: %v", err)
		return false
	}

	return open
}

func (w *TradierApiWorker) ExecuteLiveAccountPlotUpdate() {
	now := time.Now()
	nowEST := now.In(w.location)
	todayAt1615 := time.Date(nowEST.Year(), nowEST.Month(), nowEST.Day(), 16, 15, 0, 0, w.location)

	var liveAccounts []*models.LiveAccount
	if err := w.db.Where("plot_updated_at IS NULL OR plot_updated_at < ?", todayAt1615).Find(&liveAccounts).Error; err != nil {
		log.Errorf("failed to fetch live accounts: %v", err)
		return
	}

	for _, account := range liveAccounts {
		if account.BrokerName != "tradier" {
			log.Debugf("ExecuteLiveAccountPlotUpdate: skipping account %d: unsupported broker %s", account.ID, account.BrokerName)
			continue
		}

		if account.AccountType == models.LiveAccountTypeMock {
			log.Debugf("ExecuteLiveAccountPlotUpdate: skipping account %d: unsupported account type %s", account.ID, account.AccountType)
			continue
		}

		if err := w.dbService.PopulateLiveAccount(account); err != nil {
			log.Errorf("failed to populate live account: %v", err)
			continue
		}

		resp, err := account.Broker.FetchEquity()
		if err != nil {
			log.Errorf("failed to fetch equity for (%v, %v): %v", account.AccountId, account.AccountType, err)
			continue
		}

		equity := resp.Equity

		if err := w.db.Transaction(func(tx *gorm.DB) error {
			if err := w.db.Create(&models.LiveAccountPlot{
				Timestamp:     now,
				LiveAccountID: account.ID,
				Equity:        &equity,
			}).Error; err != nil {
				return fmt.Errorf("failed to create live account plot: %w", err)
			}

			if err := w.db.Model(&account).Update("plot_updated_at", todayAt1615).Error; err != nil {
				return fmt.Errorf("failed to update live account: %w", err)
			}

			return nil
		}); err != nil {
			log.Errorf("failed to create live account equity: %v", err)
			continue
		}
	}
}

func (w *TradierApiWorker) Start(ctx context.Context) {
	w.wg.Add(1)

	timer := time.NewTicker(10 * time.Second)

	log.Info("starting TradierApiWorker consumer")

	go func() {
		defer w.wg.Done()
		for {
			select {
			case <-ctx.Done():
				log.Info("stopping TradierApiWorker consumer")
				return
			case <-timer.C:
				w.updateTradierOrderQueue(ctx)
				if !w.IsMarketOpen() {
					w.ExecuteLiveAccountPlotUpdate()

					log.Debug("Market is closed: skipping live repos update")
					continue
				}

				w.ExecuteLiveReposUpdate()
			}
		}
	}()
}

func NewTradierApiWorker(wg *sync.WaitGroup, timeSalesURL, tradierNonTradesBearerToken string, polygonClient *eventservices.PolygonTickDataMachine, tradesUpdateQueue *eventmodels.FIFOQueue[*models.TradierOrderUpdateEvent], calendarURL string, db *gorm.DB, dbService models.IDatabaseService) *TradierApiWorker {
	worker := &TradierApiWorker{
		wg:                wg,
		db:                db,
		orders:            make(map[uint]*eventmodels.TradierOrder),
		timeSalesURL:      timeSalesURL,
		quotesBearerToken: tradierNonTradesBearerToken,
		polygonClient:     polygonClient,
		tradesUpdateQueue: tradesUpdateQueue,
		calendarURL:       calendarURL,
		dbService:         dbService,
	}

	var err error

	worker.location, err = time.LoadLocation("America/New_York")

	if err != nil {
		log.Fatalf("failed to load location America/New_York: %v", err)
	}

	return worker
}
