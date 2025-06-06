package router

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/data"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	pb "github.com/jiaming2012/slack-trading/src/playground"
)

type Server struct {
	cache     *models.RequestCache
	dbService *data.DatabaseService
}

func NewServer(dbService *data.DatabaseService) *Server {
	return &Server{
		cache:     models.NewRequestCache(),
		dbService: dbService,
	}
}

func convertOrders(orders []*models.OrderRecord, externalIdMap map[uint]*models.OrderRecord) []*pb.Order {
	out := make([]*pb.Order, 0)

	for _, order := range orders {
		if o := convertOrder(order, externalIdMap); o != nil {
			out = append(out, o)
		}
	}

	return out
}

func convertOrder(o *models.OrderRecord, externalIdMap map[uint]*models.OrderRecord) *pb.Order {
	var trades []*pb.Trade
	for _, trade := range o.GetTrades() {
		var orderId *uint64
		if trade.OrderID != nil {
			_orderId := uint64(*trade.OrderID)
			orderId = &_orderId
		}

		var reconcileOrderId *uint64
		if trade.ReconcileOrderID != nil {
			_reconcileOrderId := uint64(*trade.ReconcileOrderID)
			reconcileOrderId = &_reconcileOrderId
		}

		trades = append(trades, &pb.Trade{
			Id:               uint64(trade.ID),
			CreateDate:       trade.Timestamp.String(),
			Quantity:         trade.Quantity,
			Price:            trade.Price,
			OrderId:          orderId,
			ReconcileOrderId: reconcileOrderId,
		})
	}

	var closes []*pb.Order
	for _, order := range o.Closes {
		closes = append(closes, convertOrder(order, externalIdMap))
	}

	var closedBy []*pb.Trade
	for _, trade := range o.ClosedBy {
		closedBy = append(closedBy, &pb.Trade{
			CreateDate: trade.Timestamp.String(),
			Quantity:   trade.Quantity,
			Price:      trade.Price,
		})
	}

	var reconciles []*pb.Order
	for _, order := range o.Reconciles {
		reconciles = append(reconciles, convertOrder(order, externalIdMap))
	}

	var externalId *uint64
	if externalIdMap != nil {
		if reconcileOrder, ok := externalIdMap[o.ID]; ok {
			_id := uint64(*reconcileOrder.ExternalOrderID)
			externalId = &_id
		}
	} else if o.ExternalOrderID != nil {
		_externalId := uint64(*o.ExternalOrderID)
		externalId = &_externalId
	}

	previousPosition := &pb.Position{
		Quantity:          o.PreviousPosition.Quantity,
		CostBasis:         o.PreviousPosition.CostBasis,
		Pl:                o.PreviousPosition.PL,
		MaintenanceMargin: o.PreviousPosition.MaintenanceMargin,
		CurrentPrice:      o.PreviousPosition.CurrentPrice,
	}

	var closeOrderId *uint64
	if o.CloseOrderId != nil {
		_closeOrderId := uint64(*o.CloseOrderId)
		closeOrderId = &_closeOrderId
	}

	order := &pb.Order{
		Id:               uint64(o.ID),
		ExternalId:       externalId,
		ClientRequestId:  o.ClientRequestID,
		Class:            string(o.Class),
		Symbol:           o.GetInstrument().GetTicker(),
		Side:             string(o.Side),
		Quantity:         o.AbsoluteQuantity,
		Type:             string(o.OrderType),
		Duration:         string(o.Duration),
		RequestedPrice:   o.RequestedPrice,
		Tag:              o.Tag,
		Trades:           trades,
		Status:           string(o.Status),
		CreateDate:       o.Timestamp.String(),
		ClosedBy:         closedBy,
		Closes:           closes,
		Reconciles:       reconciles,
		PreviousPosition: previousPosition,
		CloseOrderId:     closeOrderId,
	}

	if o.Price != nil {
		order.Price = *o.Price
	}

	if o.StopPrice != nil {
		order.StopPrice = *o.StopPrice
	}

	if o.RejectReason != nil {
		order.RejectReason = *o.RejectReason
	}

	return order
}

func (s *Server) GetDailyTickerSummaryFromPolygon(ctx context.Context, req *pb.GetDailyTickerSummaryFromPolygonRequest) (*pb.GetDailyTickerSummaryFromPolygonResponse, error) {
	// fromTimestamp, err := time.Parse(time.RFC3339, req.TimestampRTF3339)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to parse from timestamp: %v", err)
	// }

	// from := eventmodels.NewPolygonDateFromTime(fromTimestamp)

	// toTimestamp := fromTimestamp.Add(24 * time.Hour)

	// to := eventmodels.NewPolygonDateFromTime(toTimestamp)

	// timespan := eventmodels.PolygonTimespan{
	// 	Multiplier: 1,
	// 	Unit:       eventmodels.PolygonTimespanUnitHour,
	// }

	// polygonClient := s.dbService.GetPolygonClient()
	// bars, err := polygonClient.FetchAggregateBars(eventmodels.StockSymbol(req.Symbol), timespan, from, to)
	// if err != nil {
	// 	return nil, eventmodels.NewWebError(500, "failed to fetch aggregate bars", err)
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

	return convertOrder(order, nil), nil
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

func (s *Server) GetAppVersion(ctx context.Context, req *emptypb.Empty) (*pb.GetAppVersionResponse, error) {
	return &pb.GetAppVersionResponse{
		Version: eventservices.GetAppVersion(),
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

	if playground.Meta.Environment != models.PlaygroundEnvironmentReconcile {
		return nil, fmt.Errorf("failed to get equity report: playground is not a reconciliation playground")
	}

	equityReportItems, err := playground.GetEquityReportItems(s.dbService)
	if err != nil {
		return nil, fmt.Errorf("failed to get equity report: %v", err)
	}

	var items []*pb.LiveAccountPlot
	for _, item := range equityReportItems {
		if item.Equity == nil {
			return nil, fmt.Errorf("failed to get equity report: equity is nil")
		}

		items = append(items, &pb.LiveAccountPlot{
			Timestamp: item.Timestamp.Format(time.RFC3339),
			Equity:    *item.Equity,
		})
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

	if playground.Meta.Environment != models.PlaygroundEnvironmentReconcile {
		return nil, fmt.Errorf("failed to get reconciliation report: playground is not a reconciliation playground")
	}

	// Get positions at broker
	positions, err := playground.GetLiveAccount().GetBroker().FetchPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get reconciliation report: %v", err)
	}

	var brokerPositions []*pb.PositionReport
	for _, p := range positions {
		brokerPositions = append(brokerPositions, &pb.PositionReport{
			Symbol:   p.Symbol,
			Quantity: p.Quantity,
		})
	}

	// Get reconciliation playground positions
	reconciliationPlaygroundPositionCache, err := playground.UpdatePricesAndGetPositionCache()
	if err != nil {
		return nil, fmt.Errorf("failed to get reconciliation report: %v", err)
	}

	var reconcilePositions []*pb.PositionReport
	for instrument, p := range reconciliationPlaygroundPositionCache.Iter() {
		reconcilePositions = append(reconcilePositions, &pb.PositionReport{
			Symbol:       instrument.GetTicker(),
			Quantity:     p.Quantity,
			PlaygroundId: &req.ReconcilePlaygroundId,
		})
	}

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
		for instrument, pos := range positionCache.Iter() {
			livePlaygroundPositions = append(livePlaygroundPositions, &pb.PositionReport{
				Symbol:       instrument.GetTicker(),
				Quantity:     pos.Quantity,
				PlaygroundId: &playgroundId,
			})
		}
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

		equityPlot = make([]*pb.EquityPlot, 0)
		for _, p := range plots {
			equityPlot = append(equityPlot, &pb.EquityPlot{
				CreatedAt: p.Timestamp.Format(time.RFC3339),
				Equity:    p.Value,
			})
		}
	}

	return &pb.GetAccountStatsResponse{
		EquityPlot: equityPlot,
	}, nil
}

func (s *Server) GetPlaygrounds(ctx context.Context, req *pb.GetPlaygroundsRequest) (*pb.GetPlaygroundsResponse, error) {
	playgrounds := s.dbService.GetPlaygrounds()

	playgroundsDTO := make([]*pb.PlaygroundSession, 0)
	for _, p := range playgrounds {
		if len(req.Tags) > 0 {
			meta := p.GetMeta()
			if !meta.HasTags(req.Tags) {
				continue
			}
		}

		meta := p.GetMeta()
		positionCache, err := p.UpdatePricesAndGetPositionCache()
		if err != nil {
			return nil, fmt.Errorf("failed to get playground positions: %v", err)
		}

		balance := p.GetBalance()
		equity := p.GetEquity(positionCache)
		freeMargin, err := p.GetFreeMargin()
		if err != nil {
			return nil, fmt.Errorf("failed to get playground free margin: %v", err)
		}

		positionsDTO := make(map[string]*pb.Position)
		for k, v := range positionCache.Iter() {
			positionsDTO[k.GetTicker()] = &pb.Position{
				Quantity:          v.Quantity,
				CostBasis:         v.CostBasis,
				Pl:                v.PL,
				MaintenanceMargin: v.MaintenanceMargin,
				CurrentPrice:      v.CurrentPrice,
			}
		}

		var clockStop *string
		if meta.EndAt != nil {
			_stop := meta.EndAt.Format(time.RFC3339)
			clockStop = &_stop
		}

		var repos []*pb.Repository
		for _, repo := range p.GetRepositories() {
			repos = append(repos, &pb.Repository{
				Symbol:             repo.GetSymbol().GetTicker(),
				TimespanMultiplier: uint32(repo.GetPolygonTimespan().Multiplier),
				TimespanUnit:       string(repo.GetPolygonTimespan().Unit),
				Indicators:         repo.GetIndicators(),
				HistoryInDays:      repo.GetHistoryInDays(),
			})
		}

		var liveAccountType *string
		if err := meta.LiveAccountType.Validate(); err == nil {
			liveAccountType = new(string)
			*liveAccountType = string(meta.LiveAccountType)
		}

		var reconcilePlaygroundId *string
		if p.ReconcilePlayground != nil {
			_reconcilePlaygroundId := p.ReconcilePlayground.GetId().String()
			reconcilePlaygroundId = &_reconcilePlaygroundId
		}

		playgroundsDTO = append(playgroundsDTO, &pb.PlaygroundSession{
			PlaygroundId: p.GetId().String(),
			Meta: &pb.AccountMeta{
				PlaygroundId:          p.GetId().String(),
				ReconcilePlaygroundId: reconcilePlaygroundId,
				InitialBalance:        meta.InitialBalance,
				Environment:           string(meta.Environment),
				LiveAccountType:       liveAccountType,
				Tags:                  meta.Tags,
				ClientId:              p.ClientID,
			},
			Clock: &pb.Clock{
				Start:       meta.StartAt.Format(time.RFC3339),
				Stop:        clockStop,
				CurrentTime: p.GetCurrentTime().Format(time.RFC3339),
			},
			Repositories: repos,
			Balance:      balance,
			Equity:       equity,
			FreeMargin:   freeMargin,
			Positions:    positionsDTO,
		})
	}

	return &pb.GetPlaygroundsResponse{
		Playgrounds: playgroundsDTO,
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

	return &pb.EmptyResponse{}, nil
}

func (s *Server) GetOpenOrders(ctx context.Context, req *pb.GetOpenOrdersRequest) (*pb.GetOpenOrdersResponse, error) {
	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("GetOpenOrders: failed to parse uuid: %v", err)
	}

	symbol := eventmodels.StockSymbol(req.Symbol)
	playground, err := s.dbService.GetPlayground(playgroundId)
	if err != nil {
		return nil, fmt.Errorf("GetOpenOrders: failed to get playground: %v", err)
	}

	orders := playground.GetOpenOrders(symbol)
	ordersDTO := convertOrders(orders, nil)

	return &pb.GetOpenOrdersResponse{
		Orders: ordersDTO,
	}, nil
}

func (s *Server) GetCandles(ctx context.Context, req *pb.GetCandlesRequest) (*pb.GetCandlesResponse, error) {
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

	candles, err := s.fetchCandles(playgroundId, eventmodels.StockSymbol(req.Symbol), period, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get candles: %v", err)
	}

	barsDTO := make([]*pb.Bar, 0)
	for _, c := range candles {
		barsDTO = append(barsDTO, c.ToProto())
	}

	return &pb.GetCandlesResponse{
		Bars: barsDTO,
	}, nil
}

func (s *Server) NextTick(ctx context.Context, req *pb.NextTickRequest) (*pb.TickDelta, error) {
	log.Tracef("%v: NextTick:start", req.RequestId)
	defer log.Tracef("%v: NextTick:end", req.RequestId)

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

	tick, err := s.nextTick(playgroundId, duration, req.IsPreview)
	if err != nil {
		return nil, fmt.Errorf("failed to get next tick: %v", err)
	}

	newTrades := make([]*pb.Trade, 0)
	for _, trade := range tick.NewTrades {
		var orderId *uint64
		if trade.OrderID != nil {
			_orderId := uint64(*trade.OrderID)
			orderId = &_orderId
		}

		var reconcileOrderId *uint64
		if trade.ReconcileOrderID != nil {
			_reconcileOrderId := uint64(*trade.ReconcileOrderID)
			reconcileOrderId = &_reconcileOrderId
		}

		newTrades = append(newTrades, &pb.Trade{
			Id:               uint64(trade.ID),
			CreateDate:       trade.Timestamp.String(),
			Quantity:         trade.Quantity,
			Price:            trade.Price,
			OrderId:          orderId,
			ReconcileOrderId: reconcileOrderId,
		})
	}

	newCandles := make([]*pb.Candle, 0)
	for _, c := range tick.NewCandles {
		newCandles = append(newCandles, &pb.Candle{
			Symbol: c.Symbol.GetTicker(),
			Period: int32(c.Period.Seconds()),
			Bar:    c.Bar.ToProto(),
		})
	}

	invalidOrdersDTO := convertOrders(tick.InvalidOrders, nil)

	tickDeltaEvents := make([]*pb.TickDeltaEvent, 0)
	for _, event := range tick.Events {
		var liquidationEvent *pb.LiquidationEvent

		if event.LiquidationEvent != nil {
			ordersPlaced := convertOrders(event.LiquidationEvent.OrdersPlaced, nil)

			liquidationEvent = &pb.LiquidationEvent{
				OrdersPlaced: ordersPlaced,
			}

			tickDeltaEvents = append(tickDeltaEvents, &pb.TickDeltaEvent{
				Type:             string(models.TickDeltaEventTypeLiquidation),
				LiquidationEvent: liquidationEvent,
			})
		}
	}

	tickDelta = &pb.TickDelta{
		NewTrades:          newTrades,
		NewCandles:         newCandles,
		InvalidOrders:      invalidOrdersDTO,
		Events:             tickDeltaEvents,
		CurrentTime:        tick.CurrentTime,
		IsBacktestComplete: tick.IsBacktestComplete,
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

	var sides []models.TradierOrderSide
	if req.Sides != nil {
		for _, side := range req.Sides {
			sides = append(sides, models.TradierOrderSide(side))
		}
	}

	var status []models.OrderRecordStatus
	if req.Status != nil {
		for _, s := range req.Status {
			status = append(status, models.OrderRecordStatus(s))
		}
	}

	account, err := s.dbService.GetAccount(playgroundId, req.FetchOrders, from, to, status, sides, req.Symbols)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %v", err)
	}

	positions := make(map[string]*pb.Position)
	for k, v := range account.Positions {
		positions[k] = &pb.Position{
			Quantity:          v.Quantity,
			CostBasis:         v.CostBasis,
			Pl:                v.PL,
			MaintenanceMargin: v.MaintenanceMargin,
			CurrentPrice:      v.CurrentPrice,
		}
	}

	var endAt *string
	if account.Meta.EndAt != nil {
		_endAt := account.Meta.EndAt.Format(time.RFC3339)
		endAt = &_endAt
	}

	var externalIdMap map[uint]*models.OrderRecord
	if req.FetchExternalId && account.Meta.Environment == models.PlaygroundEnvironmentLive {
		externalIdMap, err = s.dbService.FetchExternalIdMap(account.Orders)
		if err != nil {
			return nil, fmt.Errorf("failed to get external id map: %v", err)
		}
	}

	ordersDTO := convertOrders(account.Orders, externalIdMap)

	var liveAccountType *string
	if err := account.Meta.LiveAccountType.Validate(); err == nil {
		liveAccountType = new(string)
		*liveAccountType = string(account.Meta.LiveAccountType)
	}

	log.Debugf("%v: GetAccount:Orders Count: %d", requestUUID, len(ordersDTO))
	log.Tracef("%v: GetAccount:end", requestUUID)

	return &pb.GetAccountResponse{
		Meta: &pb.AccountMeta{
			PlaygroundId:          account.Meta.PlaygroundId,
			ReconcilePlaygroundId: account.Meta.ReconcilePlaygroundId,
			InitialBalance:        account.Meta.InitialBalance,
			StartDate:             account.Meta.StartAt.Format(time.RFC3339),
			EndDate:               endAt,
			Symbols:               account.Meta.Symbols,
			Environment:           string(account.Meta.Environment),
			LiveAccountType:       liveAccountType,
			Tags:                  account.Meta.Tags,
			ClientId:              account.Meta.ClientID,
		},
		Balance:    account.Balance,
		Equity:     account.Equity,
		FreeMargin: account.FreeMargin,
		Positions:  positions,
		Orders:     ordersDTO,
	}, nil
}

func (s *Server) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.Order, error) {
	log.Infof("%v: PlaceOrder:start", req.ClientRequestId)

	playgroundID, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to parse playground id: %v", err)
	}

	if req.ClientRequestId != nil {
		if order, _ := s.dbService.GetOrderByClientId(*req.ClientRequestId); order != nil {
			log.Infof("%v: PlaceOrder:Order already exists", req.ClientRequestId)

			orderDTO := convertOrder(order, nil)

			log.Infof("%v: PlaceOrder %d:end", req.ClientRequestId, order.ID)

			return orderDTO, nil
		}
	}

	var closeOrderId *uint
	if req.CloseOrderId != nil {
		closeOrderId = new(uint)
		*closeOrderId = uint(*req.CloseOrderId)
	}
	order, webErr := s.dbService.PlaceOrder(playgroundID, &models.CreateOrderRequest{
		Symbol:          req.Symbol,
		ClientRequestID: req.ClientRequestId,
		Class:           models.OrderRecordClass(req.AssetClass),
		Quantity:        req.Quantity,
		Side:            models.TradierOrderSide(req.Side),
		OrderType:       models.OrderRecordType(req.Type),
		RequestedPrice:  req.RequestedPrice,
		Price:           req.Price,
		StopPrice:       req.Sl,
		Duration:        models.OrderRecordDuration(req.Duration),
		Tag:             req.Tag,
		CloseOrderId:    closeOrderId,
		IsAdjustment:    req.IsAdjustment,
	})

	if webErr != nil {
		return nil, fmt.Errorf("failed to place order: %v", webErr)
	}

	orderDTO := convertOrder(order, nil)

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

	playgroundEnvironment := models.PlaygroundEnvironment(req.GetEnvironment())
	if err := playgroundEnvironment.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate playground environment: %v", err)
	}

	var repositoryRequests []eventmodels.CreateRepositoryRequest
	for _, repo := range req.Repositories {
		repositoryRequests = append(repositoryRequests, eventmodels.CreateRepositoryRequest{
			Symbol: repo.Symbol,
			Timespan: eventmodels.PolygonTimespanRequest{
				Multiplier: int(repo.TimespanMultiplier),
				Unit:       repo.TimespanUnit,
			},
			Source: eventmodels.RepositorySource{
				Type: eventmodels.RepositorySourceTradier,
			},
			Indicators:    repo.Indicators,
			HistoryInDays: repo.HistoryInDays,
		})
	}

	vars := models.NewLiveAccountVariables(models.LiveAccountType(req.AccountType))
	accountId, err := vars.GetTradierTradesAccountID()
	if err != nil {
		return nil, fmt.Errorf("failed to create live playground: %v", err)
	}

	source := &models.CreateAccountRequestSource{
		Broker:          req.Broker,
		LiveAccountType: models.LiveAccountType(req.AccountType),
		AccountID:       accountId,
	}

	liveAccount, found, err := s.dbService.FetchLiveAccount(source)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch live playground: %v", err)
	}

	if !found {
		return nil, fmt.Errorf("failed to fetch live playground: live account not found")
	}

	createPlaygroundReq := &models.PopulatePlaygroundRequest{
		Env:      playgroundEnvironment,
		ClientID: req.ClientId,
		Account: models.CreateAccountRequest{
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

	playground := &models.Playground{}
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

	playgroundEnvironment := models.PlaygroundEnvironment(req.GetEnvironment())
	if err := playgroundEnvironment.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate playground environment: %v", err)
	}

	var repositoryRequests []eventmodels.CreateRepositoryRequest
	for _, repo := range req.Repositories {
		repositoryRequests = append(repositoryRequests, eventmodels.CreateRepositoryRequest{
			Symbol: repo.Symbol,
			Timespan: eventmodels.PolygonTimespanRequest{
				Multiplier: int(repo.TimespanMultiplier),
				Unit:       repo.TimespanUnit,
			},
			Source: eventmodels.RepositorySource{
				Type: eventmodels.RepositorySourcePolygon,
			},
			Indicators:    repo.Indicators,
			HistoryInDays: repo.HistoryInDays,
		})
	}

	playground := &models.Playground{}
	err := s.dbService.CreatePlayground(playground, &models.PopulatePlaygroundRequest{
		Env:      playgroundEnvironment,
		ClientID: req.ClientId,
		Account: models.CreateAccountRequest{
			Balance: float64(req.Balance),
		},
		InitialBalance: float64(req.Balance),
		Clock: models.CreateClockRequest{
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

	return &pb.CreatePlaygroundResponse{
		Id: playground.GetId().String(),
	}, nil
}
