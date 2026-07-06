package router

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/twitchtv/twirp"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/jiaming2012/slack-trading/src/go/api"
	backtester_models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/backtester/safety"
	"github.com/jiaming2012/slack-trading/src/go/data"
	"github.com/jiaming2012/slack-trading/src/go/marketdata"
	"github.com/jiaming2012/slack-trading/src/go/models"
	pb "github.com/jiaming2012/slack-trading/src/go/playground"
	"github.com/jiaming2012/slack-trading/src/go/pubsub"
	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

type Server struct {
	cache            *backtester_models.RequestCache
	dbService        *data.DatabaseService
	optionsClient    *marketdata.PolygonOptionsClient
	esdbProducer     *api.EsdbProducer
	globalSignalRepo backtester_models.ISignalRepository
	simSignalRepo    backtester_models.ISignalRepository // set when a sim playground is active; WriteSignal fans out to it
}

func NewServer(optionsClient *marketdata.PolygonOptionsClient, dbService *data.DatabaseService, esdbProducer *api.EsdbProducer, globalSignalRepo backtester_models.ISignalRepository) *Server {
	return &Server{
		cache:            backtester_models.NewRequestCache(),
		dbService:        dbService,
		optionsClient:    optionsClient,
		esdbProducer:     esdbProducer,
		globalSignalRepo: globalSignalRepo,
	}
}

func (s *Server) GetDailyTickerSummaryFromPolygon(ctx context.Context, req *pb.GetDailyTickerSummaryFromPolygonRequest) (*pb.GetDailyTickerSummaryFromPolygonResponse, error) {
	// fromTimestamp, err := time.Parse(time.RFC3339, req.TimestampRTF3339)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to parse from timestamp: %v", err)
	// }

	// from := models.NewPolygonDateFromTime(fromTimestamp)

	// toTimestamp := fromTimestamp.Add(24 * time.Hour)

	// to := models.NewPolygonDateFromTime(toTimestamp)

	// timespan := models.PolygonTimespan{
	// 	Multiplier: 1,
	// 	Unit:       models.PolygonTimespanUnitHour,
	// }

	// polygonClient := s.dbService.GetPolygonClient()
	// bars, err := polygonClient.FetchAggregateBars(models.StockSymbol(req.Symbol), timespan, from, to)
	// if err != nil {
	// 	return nil, models.NewWebError(500, "failed to fetch aggregate bars", err)
	// }

	// if len(bars) == 0 {
	// 	return nil, fmt.Errorf("no bars found for symbol %s", req.Symbol)
	// }

	// for _, bar := range bars {
	// 	if bar.Timestamp.Equal(fromTimestamp) {
	// 		if bar.Timestamp.Hour() == 16 {
	// 			return &pb.GetPriceFromPolygonResponse{
	// 				TimestampRTF3339: bar.Timestamp.Format(time.RFC3339),
	// 				Price:            bar.Close,
	// 			}, nil
	// 		}
	// 	}
	// }

	// return nil, fmt.Errorf("no bars found for symbol %s", req.Symbol)
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.Order, error) {
	order, err := s.dbService.GetOrder(uint(req.OrderId))
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %v", err)
	}

	return orderToProto(order, nil), nil
}

// PERF TODO (Phase 5): For backtesting, pre-fetch all option chain data for the
// simulation's full date range during CreatePlayground. Then GetOptionsLadder
// becomes a pure in-memory lookup with near-zero latency.
func (s *Server) GetOptionsLadder(ctx context.Context, req *pb.GetOptionsLadderRequest) (*pb.GetOptionsLadderResponse, error) {
	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to get equity report: %v", err)
	}

	playground, err := s.dbService.GetPlayground(playgroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to get equity report: %v", err)
	}

	symbol := models.StockSymbol(req.StockSymbol)

	timestamp := playground.GetCurrentTime()

	var expirationInDays []int
	for _, days := range req.ExpirationInDays {
		expirationInDays = append(expirationInDays, int(days))
	}

	maxTickAge := time.Duration(float64(req.MaxTickAgeInMinutes) * float64(time.Minute))
	resp, err := s.optionsClient.FetchOptionChainV2(symbol, timestamp, int(req.MaxNoOfStrikes), req.MinDistanceBetweenStrikes, expirationInDays, maxTickAge, req.BaseStrikePrice, playground.GetCalendarRepository())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch options ladder: %v", err)
	}

	return &pb.GetOptionsLadderResponse{
		Contracts: optionLadderContractsToProto(resp.OptionContracts),
	}, nil
}

func (s *Server) MockFillOrder(ctx context.Context, req *pb.MockFillOrderRequest) (*pb.EmptyResponse, error) {
	broker, err := s.dbService.GetMockBroker(req.Broker)
	if err != nil {
		return nil, fmt.Errorf("failed to get mock broker: %v", err)
	}

	if req.DelayInSeconds != nil && *req.DelayInSeconds > 0 {
		go func() {
			time.Sleep(time.Duration(*req.DelayInSeconds) * time.Second)
			if err := broker.FillOrder(uint(req.OrderId), req.Price, string(req.Status)); err != nil {
				log.Errorf("failed to fill mock order: %v", err)
			}

			log.Debugf("Mock order %d filled, with delay", req.OrderId)
		}()
	} else {
		if err := broker.FillOrder(uint(req.OrderId), req.Price, string(req.Status)); err != nil {
			return nil, fmt.Errorf("failed to fill mock order: %v", err)
		}

		log.Debugf("Mock order %d filled, without delay", req.OrderId)
	}

	return &pb.EmptyResponse{}, nil
}

// Deprecated: RecordSignal is a telemetry-only endpoint that increments OTel counters.
// It will be replaced by WriteSignal RPC (Phase 19) which produces domain TradeSignal events.
// Keep functional until all strategies are migrated (Phase 22).
func (s *Server) RecordSignal(ctx context.Context, req *pb.RecordSignalRequest) (*pb.EmptyResponse, error) {
	clientID := ""
	if playgroundID, err := uuid.Parse(req.PlaygroundId); err == nil {
		if pg, err := s.dbService.GetPlayground(playgroundID); err == nil {
			clientID = telemetry.ClientIDOrEmpty(pg.GetClientId())
		} else {
			log.Warnf("RecordSignal: could not look up playground %s for client_id: %v", req.PlaygroundId, err)
		}
	}

	telemetry.SignalsGenerated.Add(1,
		telemetry.Label{Key: "signal_type", Value: req.SignalType},
		telemetry.Label{Key: "decision", Value: req.Decision},
		telemetry.Label{Key: "symbol", Value: req.Symbol},
		telemetry.Label{Key: "playground_id", Value: req.PlaygroundId},
		telemetry.Label{Key: "client_id", Value: clientID},
	)

	log.WithFields(log.Fields{
		"event":         "signal_generated",
		"playground_id": req.PlaygroundId,
		"signal_type":   req.SignalType,
		"direction":     req.Direction,
		"decision":      req.Decision,
		"reason":        req.Reason,
		"symbol":        req.Symbol,
		"client_id":     clientID,
	}).Info("signal recorded")

	return &pb.EmptyResponse{}, nil
}

func (s *Server) MockAddCandle(ctx context.Context, req *pb.MockAddCandleRequest) (*pb.EmptyResponse, error) {
	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("MockAddCandle: invalid playground id: %v", err)
	}

	playground, err := s.dbService.GetPlayground(playgroundId)
	if err != nil {
		return nil, fmt.Errorf("MockAddCandle: playground not found: %v", err)
	}

	if !playground.Meta.Mode.IsRealtime() {
		return nil, fmt.Errorf("MockAddCandle: only live playgrounds support mock candles")
	}

	period := time.Duration(req.PeriodInSeconds) * time.Second

	var repo *backtester_models.CandleRepository
	for _, r := range playground.GetRepositories() {
		if r.GetSymbol().GetTicker() == req.Symbol && r.GetPeriod() == period {
			repo = r
			break
		}
	}

	if repo == nil {
		return nil, fmt.Errorf("MockAddCandle: no repository for symbol=%s period=%v", req.Symbol, period)
	}

	ts, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("MockAddCandle: invalid timestamp: %v", err)
	}

	candle := &models.PolygonAggregateBarV2{
		Timestamp: ts,
		Open:      req.Open,
		High:      req.High,
		Low:       req.Low,
		Close:     req.Close,
		Volume:    req.Volume,
	}

	if _, err := repo.AppendBars([]models.ICandle{candle}); err != nil {
		return nil, fmt.Errorf("MockAddCandle: failed to append candle: %v", err)
	}

	log.Infof("MockAddCandle: appended candle for %s period=%v at %v", req.Symbol, period, ts)

	return &pb.EmptyResponse{}, nil
}

func (s *Server) GetAppVersion(ctx context.Context, req *emptypb.Empty) (*pb.GetAppVersionResponse, error) {
	return &pb.GetAppVersionResponse{
		Version: marketdata.GetAppVersion(),
	}, nil
}

func (s *Server) GetEquityReport(ctx context.Context, req *pb.GetEquityReportRequest) (*pb.GetEquityReportResponse, error) {
	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to get equity report: %v", err)
	}

	playground, err := s.dbService.GetPlayground(playgroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to get equity report: %v", err)
	}

	if !playground.Meta.IsReconciliation() {
		return nil, fmt.Errorf("failed to get equity report: playground is not a reconciliation playground")
	}

	equityReportItems, err := playground.GetEquityReportItems(s.dbService)
	if err != nil {
		return nil, fmt.Errorf("failed to get equity report: %v", err)
	}

	items, err := liveAccountPlotsToProto(equityReportItems)
	if err != nil {
		return nil, fmt.Errorf("failed to get equity report: %v", err)
	}

	return &pb.GetEquityReportResponse{
		Items: items,
	}, nil
}

func (s *Server) GetReconciliationReport(ctx context.Context, req *pb.GetReconciliationReportRequest) (*pb.GetReconciliationReportResponse, error) {
	reconcilePlaygroundId, err := uuid.Parse(req.ReconcilePlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to get reconciliation report: %v", err)
	}

	playground, err := s.dbService.GetPlayground(reconcilePlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to get reconciliation report: %v", err)
	}

	if !playground.Meta.IsReconciliation() {
		return nil, fmt.Errorf("failed to get reconciliation report: playground is not a reconciliation playground")
	}

	// Get positions at broker
	positions, err := playground.GetLiveAccount().GetBroker().FetchPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get reconciliation report: %v", err)
	}

	brokerPositions := tradierPositionReportsToProto(positions)

	// Get reconciliation playground positions
	reconciliationPlaygroundPositionCache, err := playground.UpdatePricesAndGetPositionCache()
	if err != nil {
		return nil, fmt.Errorf("failed to get reconciliation report: %v", err)
	}

	reconcilePositions := positionReportsToProto(reconciliationPlaygroundPositionCache, &req.ReconcilePlaygroundId)

	// Get live playground positions
	livePlaygrounds, err := s.dbService.GetPlaygroundsByReconcileId(reconcilePlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to get playgrounds by reconcile id: %v", err)
	}

	var livePlaygroundPositions []*pb.PositionReport
	for _, p := range livePlaygrounds {
		playgroundId := p.ID.String()
		positionCache, err := p.UpdatePricesAndGetPositionCache()
		if err != nil {
			return nil, fmt.Errorf("failed to get %s positions: %v", playgroundId, err)
		}
		livePlaygroundPositions = append(livePlaygroundPositions, positionReportsToProto(positionCache, &playgroundId)...)
	}

	return &pb.GetReconciliationReportResponse{
		BrokerPositions:         brokerPositions,
		ReconciliationPositions: reconcilePositions,
		LivePlaygroundPositions: livePlaygroundPositions,
	}, nil
}

func (s *Server) GetAccountStats(ctx context.Context, req *pb.GetAccountStatsRequest) (*pb.GetAccountStatsResponse, error) {
	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("GetAccountStats: failed to get account stats: %v", err)
	}

	var equityPlot []*pb.EquityPlot
	if req.EquityPlot {
		plots, err := s.dbService.GetAccountStatsEquity(playgroundId)
		if err != nil {
			return nil, fmt.Errorf("GetAccountStats: failed to get account stats: %v", err)
		}

		equityPlot = equityPlotsToProto(plots)
	}

	return &pb.GetAccountStatsResponse{
		EquityPlot: equityPlot,
	}, nil
}

func (s *Server) GetPlaygrounds(ctx context.Context, req *pb.GetPlaygroundsRequest) (*pb.GetPlaygroundsResponse, error) {
	playgrounds := s.dbService.GetPlaygrounds()

	type sortedPlayground struct {
		playground *pb.PlaygroundSession
		createdAt  time.Time
	}

	sortedPlaygrounds := make([]sortedPlayground, 0)
	for _, p := range playgrounds {
		if len(req.Tags) > 0 {
			meta := p.GetMeta()
			if !meta.HasTags(req.Tags) {
				continue
			}
		}

		pg, err := playgroundToProto(p)
		if err != nil {
			return nil, err
		}

		sortedPlaygrounds = append(sortedPlaygrounds, sortedPlayground{
			playground: pg,
			createdAt:  p.CreatedAt,
		})
	}

	// Sort by created at descending
	for i := 0; i < len(sortedPlaygrounds)-1; i++ {
		for j := i + 1; j < len(sortedPlaygrounds); j++ {
			if sortedPlaygrounds[i].createdAt.Before(sortedPlaygrounds[j].createdAt) {
				sortedPlaygrounds[i], sortedPlaygrounds[j] = sortedPlaygrounds[j], sortedPlaygrounds[i]
			}
		}
	}

	responsePlaygrounds := make([]*pb.PlaygroundSession, len(sortedPlaygrounds))
	for i, sp := range sortedPlaygrounds {
		responsePlaygrounds[i] = sp.playground
	}

	return &pb.GetPlaygroundsResponse{
		Playgrounds: responsePlaygrounds,
	}, nil
}

func (s *Server) DeletePlayground(ctx context.Context, req *pb.DeletePlaygroundRequest) (*pb.EmptyResponse, error) {
	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to delete playground: %v", err)
	}

	playground, err := s.dbService.GetPlayground(playgroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to delete playground: %v", err)
	}

	if playground.ReconcilePlayground != nil {
		liveRepositories := playground.GetRepositories()
		for _, repo := range liveRepositories {
			if err := s.dbService.RemoveLiveRepository(repo); err != nil {
				return nil, fmt.Errorf("failed to delete live repository: %v", err)
			}
		}
	}

	if err := s.dbService.DeletePlaygroundSession(playground); err != nil {
		return nil, fmt.Errorf("failed to delete playground session: %v", err)
	}

	if err := s.dbService.DeletePlayground(playgroundId); err != nil {
		return nil, fmt.Errorf("failed to delete playground: %v", err)
	}

	return &pb.EmptyResponse{}, nil
}

func (s *Server) SavePlayground(ctx context.Context, req *pb.SavePlaygroundRequest) (*pb.EmptyResponse, error) {
	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to save playground: %v", err)
	}

	playground, err := s.dbService.GetPlayground(playgroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to save playground: %v", err)
	}

	if err := s.dbService.SavePlayground(playground); err != nil {
		return nil, fmt.Errorf("failed to save playground: %v", err)
	}

	// Batch-persist signals to ESDB (per D-04: sim batch-on-save)
	// Sim signals go to opaque per-playground streams (not the global trade-signals stream)
	if signalRepo := playground.GetSignalRepo(); signalRepo != nil {
		signals := signalRepo.GetAll()
		simStream := models.NewSimSignalStreamName(playgroundId.String())
		for _, signal := range signals {
			signal.SetStreamName(simStream)
			pubsub.PublishAndSaveEvent(
				"grpc:SavePlayground",
				models.TradeSignalEventName,
				signal,
			)
		}
		if len(signals) > 0 {
			log.Infof("SavePlayground: published %d signals to stream %s", len(signals), simStream)
		}
	}

	return &pb.EmptyResponse{}, nil
}

func (s *Server) GetOpenOrders(ctx context.Context, req *pb.GetOpenOrdersRequest) (*pb.GetOpenOrdersResponse, error) {
	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("GetOpenOrders: failed to parse uuid: %v", err)
	}

	symbol := models.StockSymbol(req.Symbol)
	playground, err := s.dbService.GetPlayground(playgroundId)
	if err != nil {
		return nil, fmt.Errorf("GetOpenOrders: failed to get playground: %v", err)
	}

	orders := playground.GetOpenOrders(symbol)
	ordersDTO := ordersToProto(orders, nil)

	return &pb.GetOpenOrdersResponse{
		Orders: ordersDTO,
	}, nil
}

func (s *Server) GetCandlesFromDataSource(ctx context.Context, req *pb.GetCandlesRequest) (*pb.GetCandlesResponse, error) {
	panic("not implemented")
}

func (s *Server) GetCandlesFromRepo(ctx context.Context, req *pb.GetCandlesRequest) (*pb.GetCandlesResponse, error) {
	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to get next tick: %v", err)
	}

	from, err := time.Parse(time.RFC3339, req.FromRTF3339)
	if err != nil {
		return nil, fmt.Errorf("failed to get next tick while parsing from timestamp: %v", err)
	}

	var to *time.Time
	if req.ToRTF3339 != nil {
		_t, err := time.Parse(time.RFC3339, *req.ToRTF3339)
		if err != nil {
			return nil, fmt.Errorf("failed to get next tick while parsing to timestamp: %v", err)
		}

		to = &_t
	}

	period := time.Duration(req.PeriodInSeconds) * time.Second

	var calendar *models.CalendarRepository
	if req.CalculateIsExtendedHours != nil && *req.CalculateIsExtendedHours {
		_from := models.NewPolygonDateFromTime(from)
		if to == nil {
			return nil, fmt.Errorf("createClock: to date must be provided when calculating extended hours")
		}

		_to := models.NewPolygonDateFromTime(*to)
		c, err := data.FetchCalendarMap(*_from, *_to)
		if err != nil {
			return nil, fmt.Errorf("createClock: failed to fetch calendar: %w", err)
		}

		calendar = &c
	}

	var candles []*models.AggregateBarWithIndicators
	if req.Symbol[:2] == "O:" {
		candles, err = s.fetchOptionCandles(playgroundId, models.OptionSymbol(req.Symbol), period, from, to)
		if err != nil {
			return nil, fmt.Errorf("failed to get option candles: %v", err)
		}
	} else {
		candles, err = s.fetchCandles(playgroundId, models.StockSymbol(req.Symbol), period, from, to)
		if err != nil {
			return nil, fmt.Errorf("failed to get candles: %v", err)
		}
	}

	barsDTO := make([]*pb.Bar, 0)
	for _, c := range candles {
		barsDTO = append(barsDTO, c.ToProto())
		if calendar != nil {
			isExtendedHours := !calendar.IsMarketOpen(c.Timestamp)
			barsDTO[len(barsDTO)-1].IsExtendedHours = &isExtendedHours
		}
	}

	return &pb.GetCandlesResponse{
		Bars: barsDTO,
	}, nil
}
func (s *Server) NextTick(ctx context.Context, req *pb.NextTickRequest) (*pb.TickDelta, error) {
	logger := log.WithFields(log.Fields{
		"trace_id":      req.TraceId,
		"playground_id": req.PlaygroundId,
		"request_id":    req.RequestId,
	})
	logger.Trace("NextTick:start")
	defer logger.Trace("NextTick:end")

	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to get next tick: %v", err)
	}

	if len(req.RequestId) == 0 {
		return nil, fmt.Errorf("failed to get next tick: request id is empty")
	}

	var tickDelta *pb.TickDelta

	reqCh := s.cache.GetData(req.RequestId)

	isComplete := false
	defer func() {
		log.Tracef("%v: NextTick:isComplete: %v", req.RequestId, isComplete)
		if !isComplete {
			if err := s.cache.StoreData(req.RequestId, tickDelta); err != nil {
				log.Errorf("failed to store tick delta: %v", err)
			}
		}
	}()

	tickDelta = <-reqCh
	if tickDelta != nil {
		log.Debugf("Returning cached data for request id: %s", req.RequestId)
		return tickDelta, nil
	}

	duration := time.Duration(req.Seconds) * time.Second

	tickStart := time.Now()

	tick, err := s.nextTick(playgroundId, duration, req.IsPreview)
	if err != nil {
		return nil, fmt.Errorf("failed to get next tick: %v", err)
	}

	tickLatencyMs := float64(time.Since(tickStart).Milliseconds())
	logger.WithFields(log.Fields{
		"event":           "tick_processed",
		"tick_latency_ms": tickLatencyMs,
		"is_preview":      req.IsPreview,
		"duration_s":      req.Seconds,
	}).Debug("tick processed")

	tickDelta, err = tickDeltaToProto(tick)
	if err != nil {
		return nil, err
	}

	isComplete = true
	if err := s.cache.StoreData(req.RequestId, tickDelta); err != nil {
		log.Errorf("failed to store tick delta: %v", err)
	}

	return tickDelta, nil
}

func (s *Server) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.GetAccountResponse, error) {
	requestUUID := uuid.New().String()

	log.Tracef("%v: GetAccount:start", requestUUID)

	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %v", err)
	}

	var from, to *time.Time
	if req.FromRTF3339 != nil {
		_from, err := time.Parse(time.RFC3339, *req.FromRTF3339)
		if err != nil {
			return nil, fmt.Errorf("failed to get account info while parsing from timestamp: %v", err)
		}
		from = &_from
	}

	if req.ToRTF3339 != nil {
		_to, err := time.Parse(time.RFC3339, *req.ToRTF3339)
		if err != nil {
			return nil, fmt.Errorf("failed to get account info while parsing to timestamp: %v", err)
		}
		to = &_to
	}

	var sides []backtester_models.TradierOrderSide
	if req.Sides != nil {
		for _, side := range req.Sides {
			sides = append(sides, backtester_models.TradierOrderSide(side))
		}
	}

	var status []backtester_models.OrderRecordStatus
	if req.Status != nil {
		for _, s := range req.Status {
			status = append(status, backtester_models.OrderRecordStatus(s))
		}
	}

	account, err := s.dbService.GetAccount(playgroundId, req.FetchOrders, from, to, status, sides, req.Symbols)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %v", err)
	}

	var externalIdMap map[uint]*backtester_models.OrderRecord
	if req.FetchExternalId && account.Meta.Mode.IsRealtime() {
		externalIdMap, err = s.dbService.FetchExternalIdMap(account.Orders)
		if err != nil {
			return nil, fmt.Errorf("failed to get external id map: %v", err)
		}
	}

	ordersDTO := ordersToProto(account.Orders, externalIdMap)

	log.Debugf("%v: GetAccount:Orders Count: %d", requestUUID, len(ordersDTO))
	log.Tracef("%v: GetAccount:end", requestUUID)

	return accountToProto(account, ordersDTO)
}

func (s *Server) PlaceMultiLegOrder(ctx context.Context, req *pb.PlaceMultiLegOrderRequest) (*pb.PlaceMultiLegOrderResponse, error) {
	if orders, err := s.checkOrderExists(ctx, req.ClientRequestId); len(orders) > 0 || err != nil {
		if err != nil {
			return nil, fmt.Errorf("PlaceMultiLegOrder: %v", err)
		}

		return &pb.PlaceMultiLegOrderResponse{
			Orders: orders,
		}, nil
	}

	playgroundID, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to parse playground id: %v", err)
	}

	// todo: handle limit order and requested price, which is a
	// combination of multiple orders
	requests := buildMultiLegRequests(req)

	orders, err := s.dbService.PlaceOrders(playgroundID, requests)
	if err != nil {
		return nil, fmt.Errorf("PlaceMultiLegOrder: %v", err)
	}

	var resultOrders []*pb.Order
	for _, order := range orders {
		orderDTO := orderToProto(order, nil)
		resultOrders = append(resultOrders, orderDTO)

		log.Infof("%v: PlaceOrder %d:end", req.ClientRequestId, order.ID)
	}

	return &pb.PlaceMultiLegOrderResponse{
		Orders: resultOrders,
	}, nil
}

// buildMultiLegRequests converts a PlaceMultiLegOrderRequest into CreateOrderRequest
// slice, injecting a shared spread_group_key UUID and per-leg leg_role attribute.
func buildMultiLegRequests(req *pb.PlaceMultiLegOrderRequest) []*backtester_models.CreateOrderRequest {
	spreadGroupKey := uuid.New().String()

	var requests []*backtester_models.CreateOrderRequest
	for _, leg := range req.Legs {
		var closeOrderId *uint
		if leg.CloseOrderId != nil {
			closeOrderId = new(uint)
			*closeOrderId = uint(*leg.CloseOrderId)
		}

		legRole := "unknown"
		switch backtester_models.TradierOrderSide(leg.Side) {
		case backtester_models.TradierOrderSideSellToOpen, backtester_models.TradierOrderSideSellShort:
			legRole = "short"
		case backtester_models.TradierOrderSideBuyToOpen, backtester_models.TradierOrderSideBuy:
			legRole = "long"
		}

		attrs := map[string]string{
			"spread_group_key": spreadGroupKey,
			"leg_role":         legRole,
		}

		requests = append(requests, &backtester_models.CreateOrderRequest{
			Symbol:          leg.Symbol,
			ClientRequestID: req.ClientRequestId,
			Class:           backtester_models.OrderRecordClass(leg.AssetClass),
			Quantity:        leg.Quantity,
			Side:            backtester_models.TradierOrderSide(leg.Side),
			OrderType:       backtester_models.OrderRecordType(req.Type),
			Duration:        backtester_models.OrderRecordDuration(req.Duration),
			Tag:             leg.Tag,
			CloseOrderId:    closeOrderId,
			IsAdjustment:    false,
			Attributes:      attrs,
		})
	}

	return requests
}

func (s *Server) checkOrderExists(ctx context.Context, clientRequestId *string) ([]*pb.Order, error) {
	if clientRequestId != nil {
		log.Debugf("%v: checkOrderExists:start", *clientRequestId)

		orders, err := s.dbService.GetOrdersByClientId(*clientRequestId)
		if err != nil {
			return nil, fmt.Errorf("checkOrderExists: %v", err)
		}

		if len(orders) > 0 {
			var results []*pb.Order
			for _, order := range orders {
				log.Debugf("%v: checkOrderExists:Order already exists", *clientRequestId)
				orderDTO := orderToProto(order, nil)
				results = append(results, orderDTO)
			}

			return results, nil
		}
	}

	return nil, nil
}

func (s *Server) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.Order, error) {
	logger := log.WithFields(log.Fields{
		"trace_id":      req.TraceId,
		"playground_id": req.PlaygroundId,
		"symbol":        req.Symbol,
		"side":          req.Side,
	})
	logger.Info("PlaceOrder:start")

	// Kill-switch fast path: reject synchronously so the client sees the halt
	// error directly instead of an accepted order that never fills. This shares
	// the same process-wide gate consulted authoritatively at the Broker seam
	// (Playground.PlaceOrder), so it is the same single source of truth.
	if err := backtester_models.CheckOrderGate(); err != nil {
		logger.Warnf("PlaceOrder: rejected by kill switch: %v", err)
		return nil, fmt.Errorf("PlaceOrder: %w", err)
	}

	// Reserved-tag boundary (wire-companion-stops): the "companion-stop"
	// tag prefix is the recursion guard and the durable idempotency
	// association for broker-held protective stops. A client-supplied order
	// carrying it could poison the eligibility check or the restart-window
	// stop lookup, so it is rejected here at the API boundary with an
	// invalid-argument error (HTTP 400 via Twirp).
	if safety.IsCompanionStopOrderTag(req.Tag) {
		logger.Warnf("PlaceOrder: rejected reserved tag %q", req.Tag)
		return nil, twirp.InvalidArgumentError("tag", fmt.Sprintf("tag %q uses the reserved companion-stop prefix; choose a different tag", req.Tag))
	}

	if orders, err := s.checkOrderExists(ctx, req.ClientRequestId); len(orders) > 0 || err != nil {
		if err != nil {
			return nil, fmt.Errorf("PlaceOrder: %v", err)
		}

		if len(orders) > 1 {
			return nil, fmt.Errorf("PlaceOrder: multiple orders found with same client request id")
		}

		return orders[0], nil
	}

	playgroundID, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to parse playground id: %v", err)
	}

	var closeOrderId *uint
	if req.CloseOrderId != nil {
		closeOrderId = new(uint)
		*closeOrderId = uint(*req.CloseOrderId)
	}

	var signalID *uuid.UUID
	if req.SignalId != nil {
		parsed, err := uuid.Parse(*req.SignalId)
		if err != nil {
			return nil, fmt.Errorf("PlaceOrder: invalid signal_id: %w", err)
		}
		signalID = &parsed
	}

	order, webErr := s.dbService.PlaceOrder(playgroundID, &backtester_models.CreateOrderRequest{
		Symbol:          req.Symbol,
		ClientRequestID: req.ClientRequestId,
		Class:           backtester_models.OrderRecordClass(req.AssetClass),
		Quantity:        req.Quantity,
		Side:            backtester_models.TradierOrderSide(req.Side),
		OrderType:       backtester_models.OrderRecordType(req.Type),
		RequestedPrice:  req.RequestedPrice,
		Price:           req.Price,
		StopPrice:       req.Sl,
		Duration:        backtester_models.OrderRecordDuration(req.Duration),
		Tag:             req.Tag,
		CloseOrderId:    closeOrderId,
		IsAdjustment:    req.IsAdjustment,
		Attributes:      req.Attributes,
		PreviousBalance: nil,
		SignalID:        signalID,
	})

	if webErr != nil {
		return nil, fmt.Errorf("failed to place order: %v", webErr)
	}

	orderDTO := orderToProto(order, nil)

	log.Infof("%v: PlaceOrder %d:end", req.ClientRequestId, order.ID)

	return orderDTO, nil
}

func (s *Server) CreateLivePlayground(ctx context.Context, req *pb.CreateLivePlaygroundRequest) (*pb.CreatePlaygroundResponse, error) {
	if req.ClientId != nil {
		if len(*req.ClientId) == 0 {
			return nil, fmt.Errorf("failed to create live playground: client id is an empty string")
		}

		if playground := s.dbService.GetPlaygroundByClientId(*req.ClientId); playground != nil {
			return &pb.CreatePlaygroundResponse{
				Id: playground.GetId().String(),
			}, nil
		}
	}

	// Map the legacy request fields to a Mode at the router boundary:
	// combinations with no valid preset are rejected, and reconcile is not
	// an operator-selectable mode (migrate-crossed-enums).
	mode, internalReconcile, err := backtester_models.ModeFromLegacy(req.GetEnvironment(), req.AccountType)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve playground mode: %v", err)
	}

	if internalReconcile {
		return nil, fmt.Errorf("failed to create live playground: reconcile is not an operator-selectable mode")
	}

	if !mode.IsRealtime() {
		return nil, fmt.Errorf("failed to create live playground: mode %s does not use a live broker account", mode)
	}

	accountRole := backtester_models.AccountRole(req.AccountType)

	repositoryRequests := repositoryRequestsFromProto(req.Repositories, models.RepositorySourceTradier)

	vars := backtester_models.NewLiveAccountVariables(accountRole)
	accountId, err := vars.GetTradierTradesAccountID()
	if err != nil {
		return nil, fmt.Errorf("failed to create live playground: %v", err)
	}

	source := &backtester_models.CreateAccountRequestSource{
		Broker:      req.Broker,
		AccountRole: accountRole,
		AccountID:   accountId,
	}

	liveAccount, found, err := s.dbService.FetchLiveAccount(source)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch live playground: %v", err)
	}

	if !found {
		return nil, fmt.Errorf("failed to fetch live playground: live account not found")
	}

	createPlaygroundReq := &backtester_models.PopulatePlaygroundRequest{
		Mode:     mode,
		ClientID: req.ClientId,
		Account: backtester_models.CreateAccountRequest{
			Balance: float64(req.Balance),
			Source:  source,
		},
		InitialBalance: float64(req.Balance),
		Repositories:   repositoryRequests,
		Tags:           req.Tags,
		SaveToDB:       true,
		LiveAccount:    liveAccount,
	}

	createPlaygroundReq.CreatedAt = time.Now()

	playground := &backtester_models.Playground{}
	if err := s.dbService.CreatePlayground(playground, createPlaygroundReq); err != nil {
		return nil, fmt.Errorf("failed to create live playground: %v", err)
	}

	return &pb.CreatePlaygroundResponse{
		Id: playground.GetId().String(),
	}, nil
}

func (s *Server) CreatePlayground(ctx context.Context, req *pb.CreatePolygonPlaygroundRequest) (*pb.CreatePlaygroundResponse, error) {
	if req.ClientId != nil {
		if playground := s.dbService.GetPlaygroundByClientId(*req.ClientId); playground != nil {
			return &pb.CreatePlaygroundResponse{
				Id: playground.GetId().String(),
			}, nil
		}
	}

	// Map the legacy request environment to a Mode at the router boundary.
	// This entry point carries no account source, so only Simulation
	// resolves; reconcile is not operator-selectable and live playgrounds
	// go through CreateLivePlayground.
	mode, internalReconcile, err := backtester_models.ModeFromLegacy(req.GetEnvironment(), "")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve playground mode: %v", err)
	}

	if internalReconcile {
		return nil, fmt.Errorf("failed to create playground: reconcile is not an operator-selectable mode")
	}

	repositoryRequests := repositoryRequestsFromProto(req.Repositories, models.RepositorySourcePolygon)

	playground := &backtester_models.Playground{}
	err = s.dbService.CreatePlayground(playground, &backtester_models.PopulatePlaygroundRequest{
		Mode:     mode,
		ClientID: req.ClientId,
		Account: backtester_models.CreateAccountRequest{
			Balance: float64(req.Balance),
		},
		InitialBalance: float64(req.Balance),
		Clock: backtester_models.CreateClockRequest{
			StartDate: req.StartDate,
			StopDate:  req.StopDate,
		},
		Repositories: repositoryRequests,
		Tags:         req.Tags,
		SaveToDB:     false,
	})

	if err != nil {
		return nil, fmt.Errorf("s.CreatePlayground: failed to create playground: %w", err)
	}

	// Signal repository injection:
	// When replay_signal_stream is set, preload signals from ESDB into an
	// InMemorySignalRepository (replay mode). Otherwise, use environment-based
	// assignment (D-03): live/reconcile → ESDB, simulator → in-memory.
	if req.ReplaySignalStream != nil && *req.ReplaySignalStream != "" {
		if s.esdbProducer == nil {
			return nil, fmt.Errorf("s.CreatePlayground: replay_signal_stream requires ESDB producer")
		}

		// Parse date range for filtering signals
		startDate, err := time.Parse("2006-01-02", req.StartDate)
		if err != nil {
			return nil, fmt.Errorf("s.CreatePlayground: failed to parse start_date for replay: %w", err)
		}
		stopDate, err := time.Parse("2006-01-02", req.StopDate)
		if err != nil {
			return nil, fmt.Errorf("s.CreatePlayground: failed to parse stop_date for replay: %w", err)
		}

		// Read all signals from ESDB, filter to date range, load into in-memory repo
		tempRepo := backtester_models.NewESDBSignalRepository(s.esdbProducer)
		allSignals := tempRepo.GetAll()

		replayRepo := backtester_models.NewInMemorySignalRepository()
		var loaded int
		for _, sig := range allSignals {
			if !sig.Timestamp.Before(startDate) && sig.Timestamp.Before(stopDate.AddDate(0, 0, 1)) {
				if err := replayRepo.Write(sig); err != nil {
					return nil, fmt.Errorf("s.CreatePlayground: failed to write replay signal: %w", err)
				}
				loaded++
			}
		}

		playground.SetSignalRepo(replayRepo)
		log.Infof("CreatePlayground: replay mode, loaded %d signals from ESDB stream (total in stream: %d)", loaded, len(allSignals))
	} else {
		if mode.IsRealtime() {
			if s.esdbProducer != nil {
				playground.SetSignalRepo(backtester_models.NewESDBSignalRepository(s.esdbProducer))
			}
		} else {
			simRepo := backtester_models.NewInMemorySignalRepository()
			playground.SetSignalRepo(simRepo)
			s.simSignalRepo = simRepo
		}
	}

	log.Infof("CreatePlayground: id=%s mode=%s balance=%.2f start=%s stop=%s",
		playground.GetId(), mode, req.Balance, req.StartDate, req.StopDate)

	return &pb.CreatePlaygroundResponse{
		Id: playground.GetId().String(),
	}, nil
}

func filterSignals(signals []*models.TradeSignal, name *string, symbol *string, startTime *time.Time, endTime *time.Time) []*pb.TradeSignalProto {
	var result []*pb.TradeSignalProto
	for _, s := range signals {
		if name != nil && string(s.Name) != *name {
			continue
		}
		if symbol != nil && string(s.Symbol) != *symbol {
			continue
		}
		if startTime != nil && s.Timestamp.Before(*startTime) {
			continue
		}
		if endTime != nil && s.Timestamp.After(*endTime) {
			continue
		}
		result = append(result, tradeSignalToProto(s))
	}
	if result == nil {
		result = []*pb.TradeSignalProto{}
	}
	return result
}

func (s *Server) WriteSignal(ctx context.Context, req *pb.WriteSignalRequest) (*pb.WriteSignalResponse, error) {
	signalName := models.SignalName(req.Name)
	if err := signalName.Validate(); err != nil {
		return nil, fmt.Errorf("WriteSignal: invalid signal name %q: %w", req.Name, err)
	}

	if req.Symbol == "" {
		return nil, fmt.Errorf("WriteSignal: symbol is required")
	}

	if req.Timestamp == nil {
		return nil, fmt.Errorf("WriteSignal: timestamp is required")
	}

	attrs := make(map[string]interface{}, len(req.Attributes))
	for k, v := range req.Attributes {
		attrs[k] = v
	}

	signal := models.NewTradeSignal(signalName, models.StockSymbol(req.Symbol), req.Timestamp.AsTime(), attrs)

	// When a sim playground is active, write to its in-memory repo first
	// (the primary store for sim). Global repo write is best-effort since
	// ESDB may not be available in dev.
	if s.simSignalRepo != nil {
		if err := s.simSignalRepo.Write(signal); err != nil {
			return nil, fmt.Errorf("WriteSignal: failed to write signal to sim repo: %w", err)
		}
	}

	if err := s.globalSignalRepo.Write(signal); err != nil {
		if s.simSignalRepo != nil {
			// Sim repo already has the signal — ESDB failure is non-fatal
			log.Warnf("WriteSignal: global repo write failed (non-fatal in sim mode): %v", err)
		} else {
			return nil, fmt.Errorf("WriteSignal: failed to write signal: %w", err)
		}
	}

	telemetry.SignalsGenerated.Add(1,
		telemetry.Label{Key: "signal_type", Value: req.Name},
		telemetry.Label{Key: "symbol", Value: req.Symbol},
	)

	log.Infof("WriteSignal: stored signal %s (name=%s, symbol=%s)", signal.ID, req.Name, req.Symbol)

	return &pb.WriteSignalResponse{
		SignalId: signal.ID.String(),
	}, nil
}

func (s *Server) GetSignals(ctx context.Context, req *pb.GetSignalsRequest) (*pb.GetSignalsResponse, error) {
	allSignals := s.globalSignalRepo.GetAll()

	var name *string
	if req.Name != nil {
		name = req.Name
	}
	var symbol *string
	if req.Symbol != nil {
		symbol = req.Symbol
	}
	var startTime *time.Time
	if req.StartTime != nil {
		t := req.StartTime.AsTime()
		startTime = &t
	}
	var endTime *time.Time
	if req.EndTime != nil {
		t := req.EndTime.AsTime()
		endTime = &t
	}

	filtered := filterSignals(allSignals, name, symbol, startTime, endTime)

	return &pb.GetSignalsResponse{
		Signals: filtered,
	}, nil
}

func (s *Server) GetProcessedSignals(ctx context.Context, req *pb.GetProcessedSignalsRequest) (*pb.GetProcessedSignalsResponse, error) {
	if req.PlaygroundId == "" {
		return nil, fmt.Errorf("GetProcessedSignals: playground_id is required")
	}

	playgroundID, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("GetProcessedSignals: invalid playground_id %q: %w", req.PlaygroundId, err)
	}

	pg, err := s.dbService.FetchPlayground(playgroundID)
	if err != nil {
		return nil, fmt.Errorf("GetProcessedSignals: playground not found: %w", err)
	}

	consumedSignals := pg.GetSignalRepo().GetAll()

	var name *string
	if req.Name != nil {
		name = req.Name
	}
	var symbol *string
	if req.Symbol != nil {
		symbol = req.Symbol
	}
	var startTime *time.Time
	if req.StartTime != nil {
		t := req.StartTime.AsTime()
		startTime = &t
	}
	var endTime *time.Time
	if req.EndTime != nil {
		t := req.EndTime.AsTime()
		endTime = &t
	}

	filtered := filterSignals(consumedSignals, name, symbol, startTime, endTime)

	// Build signal_id → order_ids map from the playground's orders
	signalOrderMap := make(map[string][]uint64)
	for _, order := range pg.GetAllOrders() {
		if order.SignalID != nil {
			sid := order.SignalID.String()
			signalOrderMap[sid] = append(signalOrderMap[sid], uint64(order.ID))
		}
	}

	// Enrich signals with their linked order IDs
	for _, sig := range filtered {
		if orderIDs, ok := signalOrderMap[sig.Id]; ok {
			sig.OrderIds = orderIDs
		}
	}

	return &pb.GetProcessedSignalsResponse{
		Signals: filtered,
	}, nil
}
