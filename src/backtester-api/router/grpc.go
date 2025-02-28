package router

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/backtester-api/services"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	pb "github.com/jiaming2012/slack-trading/src/playground"
)

type Server struct {
	cache *models.RequestCache
}

func NewServer() *Server {
	return &Server{
		cache: models.NewRequestCache(),
	}
}

func convertOrders(orders []*models.BacktesterOrder) []*pb.Order {
	out := make([]*pb.Order, 0)

	for _, o := range orders {
		out = append(out, convertOrder(o))
	}

	return out
}

func convertOrder(o *models.BacktesterOrder) *pb.Order {
	var trades []*pb.Trade
	for _, trade := range o.Trades {
		trades = append(trades, &pb.Trade{
			Symbol:     trade.GetSymbol().GetTicker(),
			CreateDate: trade.Timestamp.String(),
			Quantity:   trade.Quantity,
			Price:      trade.Price,
		})
	}

	var closes []*pb.Order
	for _, order := range o.Closes {
		closes = append(closes, convertOrder(order))
	}

	var closedBy []*pb.Trade
	for _, trade := range o.ClosedBy {
		closedBy = append(closedBy, &pb.Trade{
			Symbol:     trade.GetSymbol().GetTicker(),
			CreateDate: trade.Timestamp.String(),
			Quantity:   trade.Quantity,
			Price:      trade.Price,
		})
	}
	order := &pb.Order{
		Id:             uint64(o.ID),
		Class:          string(o.Class),
		Symbol:         o.Symbol.GetTicker(),
		Side:           string(o.Side),
		Quantity:       o.AbsoluteQuantity,
		Type:           string(o.Type),
		Duration:       string(o.Duration),
		RequestedPrice: o.RequestedPrice,
		Tag:            o.Tag,
		Trades:         trades,
		Status:         string(o.Status),
		CreateDate:     o.CreateDate.String(),
		ClosedBy:       closedBy,
		Closes:         closes,
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

func (s *Server) GetAppVersion(ctx context.Context, req *emptypb.Empty) (*pb.GetAppVersionResponse, error) {
	return &pb.GetAppVersionResponse{
		Version: eventservices.GetAppVersion(),
	}, nil
}

func (s *Server) GetAccountStats(ctx context.Context, req *pb.GetAccountStatsRequest) (*pb.GetAccountStatsResponse, error) {
	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("GetAccountStats: failed to get account stats: %v", err)
	}

	var equityPlot []*pb.EquityPlot
	if req.EquityPlot {
		plots, err := getAccountStatsEquity(playgroundId)
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
	playgrounds := GetPlaygrounds()

	playgroundsDTO := make([]*pb.PlaygroundSession, 0)
	for _, p := range playgrounds {
		if len(req.Tags) > 0 {
			meta := p.GetMeta()
			if !meta.HasTags(req.Tags) {
				continue
			}
		}

		meta := p.GetMeta()
		positions, err := p.GetPositions()
		if err != nil {
			return nil, fmt.Errorf("failed to get playground positions: %v", err)
		}

		balance := p.GetBalance()
		equity := p.GetEquity(positions)
		freeMargin, err := p.GetFreeMargin()
		if err != nil {
			return nil, fmt.Errorf("failed to get playground free margin: %v", err)
		}

		positionsDTO := make(map[string]*pb.Position)
		for k, v := range positions {
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

		playgroundsDTO = append(playgroundsDTO, &pb.PlaygroundSession{
			PlaygroundId: p.GetId().String(),
			Meta: &pb.Meta{
				InitialBalance:  meta.InitialBalance,
				Environment:     string(meta.Environment),
				LiveAccountType: liveAccountType,
				Tags:            meta.Tags,
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

	playground, err := getPlayground(playgroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to delete playground: %v", err)
	}

	if livePlayground, ok := playground.(*models.LivePlayground); ok {
		liveRepositories := livePlayground.GetRepositories()
		for _, repo := range liveRepositories {
			if err := services.RemoveLiveRepository(repo); err != nil {
				return nil, fmt.Errorf("failed to delete live repository: %v", err)
			}
		}
	}

	if err := deletePlaygroundSession(playground); err != nil {
		return nil, fmt.Errorf("failed to delete playground session: %v", err)
	}

	if err := deletePlayground(playgroundId); err != nil {
		return nil, fmt.Errorf("failed to delete playground: %v", err)
	}

	return &pb.EmptyResponse{}, nil
}

func (s *Server) SavePlayground(ctx context.Context, req *pb.SavePlaygroundRequest) (*pb.EmptyResponse, error) {
	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to save playground: %v", err)
	}

	playground, err := getPlayground(playgroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to save playground: %v", err)
	}

	if err := savePlayground(playground); err != nil {
		return nil, fmt.Errorf("failed to save playground: %v", err)
	}

	return &pb.EmptyResponse{}, nil
}

func (s *Server) GetOpenOrders(ctx context.Context, req *pb.GetOpenOrdersRequest) (*pb.GetOpenOrdersResponse, error) {
	playgroundId, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %v", err)
	}

	symbol := eventmodels.StockSymbol(req.Symbol)
	orders, err := getOpenOrders(playgroundId, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %v", err)
	}

	ordersDTO := convertOrders(orders)

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

	to, err := time.Parse(time.RFC3339, req.ToRTF3339)
	if err != nil {
		return nil, fmt.Errorf("failed to get next tick while parsing to timestamp: %v", err)
	}

	period := time.Duration(req.PeriodInSeconds) * time.Second

	candles, err := fetchCandles(playgroundId, eventmodels.StockSymbol(req.Symbol), period, from, to)
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

	tick, err := nextTick(playgroundId, duration, req.IsPreview)
	if err != nil {
		return nil, fmt.Errorf("failed to get next tick: %v", err)
	}

	newTrades := make([]*pb.Trade, 0)
	for _, trade := range tick.NewTrades {
		newTrades = append(newTrades, &pb.Trade{
			Symbol:     trade.GetSymbol().GetTicker(),
			CreateDate: trade.Timestamp.String(),
			Quantity:   trade.Quantity,
			Price:      trade.Price,
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

	invalidOrdersDTO := convertOrders(tick.InvalidOrders)

	tickDeltaEvents := make([]*pb.TickDeltaEvent, 0)
	for _, event := range tick.Events {
		var liquidationEvent *pb.LiquidationEvent

		if event.LiquidationEvent != nil {
			ordersPlaced := convertOrders(event.LiquidationEvent.OrdersPlaced)

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

	var status []models.BacktesterOrderStatus
	if req.Status != nil {
		for _, s := range req.Status {
			status = append(status, models.BacktesterOrderStatus(s))
		}
	}

	account, err := getAccountInfo(playgroundId, req.FetchOrders, from, to, status, sides)
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

	ordersDTO := convertOrders(account.Orders)

	var liveAccountType *string
	if err := account.Meta.LiveAccountType.Validate(); err == nil {
		liveAccountType = new(string)
		*liveAccountType = string(account.Meta.LiveAccountType)
	}

	return &pb.GetAccountResponse{
		Meta: &pb.AccountMeta{
			InitialBalance:  account.Meta.InitialBalance,
			StartDate:       account.Meta.StartAt.Format(time.RFC3339),
			EndDate:         endAt,
			Symbols:         account.Meta.Symbols,
			Environment:     string(account.Meta.Environment),
			LiveAccountType: liveAccountType,
			Tags:            account.Meta.Tags,
		},
		Balance:    account.Balance,
		Equity:     account.Equity,
		FreeMargin: account.FreeMargin,
		Positions:  positions,
		Orders:     ordersDTO,
	}, nil
}

func (s *Server) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.Order, error) {
	playgroundID, err := uuid.Parse(req.PlaygroundId)
	if err != nil {
		return nil, fmt.Errorf("failed to parse playground id: %v", err)
	}

	order, webErr := placeOrder(playgroundID, &CreateOrderRequest{
		Symbol:         req.Symbol,
		Class:          models.BacktesterOrderClass(req.AssetClass),
		Quantity:       req.Quantity,
		Side:           models.TradierOrderSide(req.Side),
		OrderType:      models.BacktesterOrderType(req.Type),
		RequestedPrice: req.RequestedPrice,
		Price:          req.Price,
		StopPrice:      nil,
		Duration:       models.BacktesterOrderDuration(req.Duration),
		Tag:            req.Tag,
	})

	if webErr != nil {
		return nil, fmt.Errorf("failed to place order: %v", webErr)
	}

	orderDTO := convertOrder(order)

	return orderDTO, nil
}

func (s *Server) CreateLivePlayground(ctx context.Context, req *pb.CreateLivePlaygroundRequest) (*pb.CreatePlaygroundResponse, error) {
	if req.ClientId != nil {
		if playground := getPlaygroundByClientId(*req.ClientId); playground != nil {
			return &pb.CreatePlaygroundResponse{
				Id: playground.GetId().String(),
			}, nil
		}
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

	createPlaygroundReq := &CreatePlaygroundRequest{
		Env:      req.GetEnvironment(),
		ClientID: req.ClientId,
		Account: CreateAccountRequest{
			Balance: float64(req.Balance),
			Source: &models.CreateAccountRequestSource{
				Broker:      req.Broker,
				AccountType: models.LiveAccountType(req.AccountType),
				AccountID:   accountId,
			},
		},
		InitialBalance: float64(req.Balance),
		Repositories:   repositoryRequests,
		Tags:           req.Tags,
		SaveToDB:       true,
	}

	createPlaygroundReq.CreatedAt = time.Now()

	playground, _, err := CreatePlayground(createPlaygroundReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create playground: %v", err)
	}

	return &pb.CreatePlaygroundResponse{
		Id: playground.GetId().String(),
	}, nil
}

func (s *Server) CreatePlayground(ctx context.Context, req *pb.CreatePolygonPlaygroundRequest) (*pb.CreatePlaygroundResponse, error) {
	if req.ClientId != nil {
		if playground := getPlaygroundByClientId(*req.ClientId); playground != nil {
			return &pb.CreatePlaygroundResponse{
				Id: playground.GetId().String(),
			}, nil
		}
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

	playground, _, err := CreatePlayground(&CreatePlaygroundRequest{
		Env:      req.GetEnvironment(),
		ClientID: req.ClientId,
		Account: CreateAccountRequest{
			Balance: float64(req.Balance),
		},
		InitialBalance: float64(req.Balance),
		Clock: CreateClockRequest{
			StartDate: req.StartDate,
			StopDate:  req.StopDate,
		},
		Repositories: repositoryRequests,
		Tags:         req.Tags,
		SaveToDB:     false,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create playground: %w", err)
	}

	return &pb.CreatePlaygroundResponse{
		Id: playground.GetId().String(),
	}, nil
}
